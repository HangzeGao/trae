#!/usr/bin/env python3
"""
Train SkySense++ model on 38-Cloud dataset and generate validation visualizations.
"""

import argparse
import sys
from pathlib import Path
from tqdm import tqdm
import matplotlib.pyplot as plt
import numpy as np

import torch
import torch.nn as nn
import torch.optim as optim
from torch.utils.data import DataLoader, Subset
import torchvision.transforms as transforms
from PIL import Image

# Add project root to path
project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

from utils import load_config, set_seed, get_device, save_checkpoint, count_parameters
from models.skysense_pp import SkySensePPModel
from models.losses import get_loss_function_from_config
from models.metrics import SegmentationMetrics


class CloudDataset(torch.utils.data.Dataset):
    """Dataset loader for 38-Cloud dataset."""
    
    def __init__(self, data_dir, transform=None, target_transform=None):
        """
        Args:
            data_dir: Path to the dataset directory
            transform: Transform for input images
            target_transform: Transform for masks
        """
        self.data_dir = Path(data_dir)
        self.transform = transform
        self.target_transform = target_transform
        
        # Find all image patches
        self.image_files = sorted(self._find_images())
        
        if len(self.image_files) == 0:
            raise ValueError(f"No images found in {data_dir}")
        
        print(f"Found {len(self.image_files)} images in {data_dir}")
    
    def _find_images(self):
        """Find all image files in the dataset directory."""
        extensions = ['.TIF', '.tif', '.TIFF', '.tiff', '.png', '.PNG']
        image_files = []
        
        for ext in extensions:
            image_files.extend(list(self.data_dir.rglob(f"*{ext}")))
        
        return [f for f in image_files if 'gt' not in str(f).lower()]
    
    def __len__(self):
        return len(self.image_files)
    
    def __getitem__(self, idx):
        img_path = self.image_files[idx]
        
        # Load image
        image = Image.open(img_path)
        
        # Load corresponding mask
        mask_path = str(img_path).replace('.TIF', '_gt.TIF').replace('.tif', '_gt.tif')
        if not Path(mask_path).exists():
            mask_path = str(img_path).replace('.TIFF', '_gt.TIFF').replace('.tiff', '_gt.tiff')
        if not Path(mask_path).exists():
            mask_path = str(img_path).replace('.png', '_gt.png').replace('.PNG', '_gt.PNG')
        
        mask = Image.open(mask_path).convert('L')
        
        # Apply transforms
        if self.transform:
            image = self.transform(image)
        if self.target_transform:
            mask = self.target_transform(mask)
        
        return image, mask


def visualize_predictions(model, dataloader, device, save_dir, num_samples=5, prefix='val'):
    """
    Visualize model predictions.
    
    Args:
        model: Trained model
        dataloader: Validation dataloader
        device: Device to run on
        save_dir: Directory to save visualizations
        num_samples: Number of samples to visualize
        prefix: Prefix for save filenames
    """
    save_dir = Path(save_dir)
    save_dir.mkdir(parents=True, exist_ok=True)
    
    model.eval()
    
    samples_saved = 0
    
    with torch.no_grad():
        for batch_idx, (images, masks) in enumerate(dataloader):
            if samples_saved >= num_samples:
                break
            
            images = images.to(device)
            outputs = model(images)
            
            batch_size = images.size(0)
            for i in range(batch_size):
                if samples_saved >= num_samples:
                    break
                
                # Get single sample
                img = images[i].cpu().numpy().transpose(1, 2, 0)
                mask = masks[i].cpu().numpy()
                pred = outputs[i, 0].cpu().numpy()
                
                # Normalize image
                img = (img - img.min()) / (img.max() - img.min() + 1e-8)
                
                # Create figure
                fig, axes = plt.subplots(1, 4, figsize=(16, 4))
                
                # RGB composite
                if img.shape[2] >= 3:
                    axes[0].imshow(img[:, :, :3])
                    axes[0].set_title('Input (RGB)')
                else:
                    axes[0].imshow(img[:, :, 0], cmap='gray')
                    axes[0].set_title('Input')
                axes[0].axis('off')
                
                # NIR if available
                if img.shape[2] >= 4:
                    axes[1].imshow(img[:, :, 3], cmap='gray')
                    axes[1].set_title('NIR Channel')
                else:
                    axes[1].axis('off')
                    axes[1].set_title('N/A')
                axes[1].axis('off')
                
                # Ground truth
                axes[2].imshow(mask, cmap='gray', vmin=0, vmax=1)
                axes[2].set_title('Ground Truth')
                axes[2].axis('off')
                
                # Prediction
                axes[3].imshow(pred, cmap='gray', vmin=0, vmax=1)
                axes[3].set_title('Prediction')
                axes[3].axis('off')
                
                plt.tight_layout()
                
                # Save figure
                save_path = save_dir / f'{prefix}_sample_{samples_saved:03d}.png'
                plt.savefig(save_path, dpi=150, bbox_inches='tight')
                plt.close()
                
                print(f"Saved visualization: {save_path}")
                samples_saved += 1
    
    print(f"\nSaved {samples_saved} validation visualizations to {save_dir}")


def create_comparison_figure(model, dataset, device, save_path, num_samples=6):
    """
    Create a comprehensive comparison figure with multiple samples.
    
    Args:
        model: Trained model
        dataset: Dataset to sample from
        device: Device to run on
        save_path: Path to save the figure
        num_samples: Number of samples to display
    """
    model.eval()
    
    indices = np.random.choice(len(dataset), min(num_samples, len(dataset)), replace=False)
    
    fig, axes = plt.subplots(num_samples, 4, figsize=(16, 4 * num_samples))
    
    if num_samples == 1:
        axes = axes.reshape(1, -1)
    
    for row, idx in enumerate(indices):
        images, masks = dataset[idx]
        
        with torch.no_grad():
            img_tensor = images.unsqueeze(0).to(device)
            output = model(img_tensor)
            pred = output[0, 0].cpu().numpy()
        
        img = images.numpy().transpose(1, 2, 0)
        img = (img - img.min()) / (img.max() - img.min() + 1e-8)
        mask = masks.numpy()
        
        # RGB composite
        if img.shape[2] >= 3:
            axes[row, 0].imshow(img[:, :, :3])
        else:
            axes[row, 0].imshow(img[:, :, 0], cmap='gray')
        axes[row, 0].set_title('Input' if row == 0 else '')
        axes[row, 0].axis('off')
        
        # Ground truth
        axes[row, 1].imshow(mask, cmap='gray', vmin=0, vmax=1)
        axes[row, 1].set_title('Ground Truth' if row == 0 else '')
        axes[row, 1].axis('off')
        
        # Prediction
        axes[row, 2].imshow(pred, cmap='gray', vmin=0, vmax=1)
        axes[row, 2].set_title('SkySense++ Prediction' if row == 0 else '')
        axes[row, 2].axis('off')
        
        # Overlay
        overlay = img[:, :, :3].copy()
        if pred.shape == overlay.shape[:2]:
            overlay[pred > 0.5, 0] = 1.0
            overlay[pred > 0.5, 1] = 0.0
            overlay[pred > 0.5, 2] = 0.0
        axes[row, 3].imshow(overlay)
        axes[row, 3].set_title('Overlay (Red=Cloud)' if row == 0 else '')
        axes[row, 3].axis('off')
    
    plt.tight_layout()
    plt.savefig(save_path, dpi=150, bbox_inches='tight')
    plt.close()
    
    print(f"Comparison figure saved: {save_path}")


def train_skysensepp(
    data_dir,
    epochs=10,
    batch_size=8,
    learning_rate=0.001,
    save_dir='./checkpoints',
    val_split=0.2,
    num_workers=0
):
    """
    Train SkySense++ model on cloud segmentation dataset.
    
    Args:
        data_dir: Path to dataset directory
        epochs: Number of training epochs
        batch_size: Batch size for training
        learning_rate: Learning rate
        save_dir: Directory to save checkpoints and visualizations
        val_split: Fraction of data to use for validation
        num_workers: Number of data loading workers
    """
    set_seed(42)
    device = get_device()
    
    print("=" * 80)
    print("SKYSENSE++ MODEL TRAINING ON 38-CLOUD DATASET")
    print("=" * 80)
    print(f"Device: {device}")
    print(f"Data directory: {data_dir}")
    print(f"Epochs: {epochs}")
    print(f"Batch size: {batch_size}")
    print(f"Learning rate: {learning_rate}")
    print("=" * 80)
    
    # Transforms
    train_transform = transforms.Compose([
        transforms.Resize((256, 256)),
        transforms.ToTensor(),
    ])
    
    val_transform = transforms.Compose([
        transforms.Resize((256, 256)),
        transforms.ToTensor(),
    ])
    
    target_transform = transforms.Compose([
        transforms.Resize((256, 256)),
        transforms.ToTensor(),
    ])
    
    # Load dataset
    print("\nLoading dataset...")
    full_dataset = CloudDataset(
        data_dir,
        transform=train_transform,
        target_transform=target_transform
    )
    
    # Split dataset
    dataset_size = len(full_dataset)
    indices = list(range(dataset_size))
    split = int(np.floor(val_split * dataset_size))
    
    np.random.shuffle(indices)
    train_indices = indices[split:]
    val_indices = indices[:split]
    
    train_dataset = Subset(full_dataset, train_indices)
    val_dataset = Subset(full_dataset, val_indices)
    
    print(f"Training samples: {len(train_dataset)}")
    print(f"Validation samples: {len(val_dataset)}")
    
    # Create dataloaders
    train_loader = DataLoader(
        train_dataset,
        batch_size=batch_size,
        shuffle=True,
        num_workers=num_workers,
        pin_memory=True if device.type == 'cuda' else False
    )
    
    val_loader = DataLoader(
        val_dataset,
        batch_size=batch_size,
        shuffle=False,
        num_workers=num_workers,
        pin_memory=True if device.type == 'cuda' else False
    )
    
    # Create model
    print("\nCreating SkySense++ model...")
    model = SkySensePPModel(
        encoder_name='resnet34',
        in_channels=4,
        num_classes=1,
        fusion_type='adaptive',
        use_semantic_enhancement=True
    ).to(device)
    
    print(f"Model parameters: {count_parameters(model):,}")
    
    # Loss and optimizer
    criterion = nn.BCELoss()
    optimizer = optim.Adam(model.parameters(), lr=learning_rate)
    scheduler = optim.lr_scheduler.ReduceLROnPlateau(optimizer, mode='min', patience=3, factor=0.5)
    
    metrics = SegmentationMetrics()
    
    # Training loop
    save_dir = Path(save_dir)
    save_dir.mkdir(parents=True, exist_ok=True)
    
    best_val_loss = float('inf')
    
    print("\nStarting training...")
    print("=" * 80)
    
    for epoch in range(epochs):
        # Training phase
        model.train()
        train_loss = 0.0
        train_pbar = tqdm(train_loader, desc=f'Epoch {epoch+1}/{epochs} [Train]')
        
        for images, masks in train_pbar:
            images = images.to(device)
            masks = masks.to(device)
            
            optimizer.zero_grad()
            
            outputs = model(images)
            loss = criterion(outputs, masks)
            
            loss.backward()
            optimizer.step()
            
            train_loss += loss.item()
            train_pbar.set_postfix({'loss': f'{loss.item():.4f}'})
        
        train_loss /= len(train_loader)
        
        # Validation phase
        model.eval()
        val_loss = 0.0
        val_dice = 0.0
        val_iou = 0.0
        val_pbar = tqdm(val_loader, desc=f'Epoch {epoch+1}/{epochs} [Val]')
        
        with torch.no_grad():
            for images, masks in val_pbar:
                images = images.to(device)
                masks = masks.to(device)
                
                outputs = model(images)
                loss = criterion(outputs, masks)
                batch_metrics = metrics(outputs, masks)
                
                val_loss += loss.item()
                val_dice += batch_metrics['dice']
                val_iou += batch_metrics['iou']
                
                val_pbar.set_postfix({
                    'loss': f'{loss.item():.4f}',
                    'dice': f'{batch_metrics["dice"]:.4f}'
                })
        
        val_loss /= len(val_loader)
        val_dice /= len(val_loader)
        val_iou /= len(val_loader)
        
        scheduler.step(val_loss)
        
        print(f"\nEpoch {epoch+1}/{epochs}:")
        print(f"  Train Loss: {train_loss:.4f}")
        print(f"  Val Loss: {val_loss:.4f}")
        print(f"  Val Dice: {val_dice:.4f}")
        print(f"  Val IoU: {val_iou:.4f}")
        
        # Save best model
        if val_loss < best_val_loss:
            best_val_loss = val_loss
            checkpoint_path = save_dir / 'best_skysensepp_model.pth'
            torch.save({
                'epoch': epoch,
                'model_state_dict': model.state_dict(),
                'optimizer_state_dict': optimizer.state_dict(),
                'val_loss': val_loss,
                'val_dice': val_dice,
                'val_iou': val_iou
            }, checkpoint_path)
            print(f"  ✓ Saved best model: {checkpoint_path}")
        
        # Generate visualizations every 5 epochs
        if (epoch + 1) % 5 == 0 or epoch == epochs - 1:
            vis_dir = save_dir / 'visualizations'
            visualize_predictions(
                model, val_loader, device, vis_dir,
                num_samples=3, prefix=f'epoch_{epoch+1}'
            )
    
    print("\n" + "=" * 80)
    print("TRAINING COMPLETED")
    print("=" * 80)
    
    # Final visualization
    print("\nGenerating final validation visualizations...")
    final_vis_dir = save_dir / 'final_visualizations'
    
    visualize_predictions(
        model, val_loader, device, final_vis_dir,
        num_samples=6, prefix='final'
    )
    
    # Create comprehensive comparison figure
    comparison_path = final_vis_dir / 'comparison_figure.png'
    create_comparison_figure(
        model, val_dataset, device, comparison_path, num_samples=6
    )
    
    print(f"\n✓ All visualizations saved to: {final_vis_dir}")
    print(f"✓ Best model checkpoint: {checkpoint_path}")
    print(f"✓ Training complete!")
    
    return model, checkpoint_path


if __name__ == '__main__':
    import argparse
    
    parser = argparse.ArgumentParser(description='Train SkySense++ on 38-Cloud dataset')
    parser.add_argument('--data_dir', type=str, required=True, help='Path to 38-Cloud dataset')
    parser.add_argument('--epochs', type=int, default=10, help='Number of training epochs')
    parser.add_argument('--batch_size', type=int, default=8, help='Batch size')
    parser.add_argument('--lr', type=float, default=0.001, help='Learning rate')
    parser.add_argument('--save_dir', type=str, default='./checkpoints', help='Save directory')
    parser.add_argument('--val_split', type=float, default=0.2, help='Validation split ratio')
    
    args = parser.parse_args()
    
    train_skysensepp(
        data_dir=args.data_dir,
        epochs=args.epochs,
        batch_size=args.batch_size,
        learning_rate=args.lr,
        save_dir=args.save_dir,
        val_split=args.val_split
    )
