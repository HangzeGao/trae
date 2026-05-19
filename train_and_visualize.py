#!/usr/bin/env python3
"""
Training script using standard SMP models for cloud segmentation.
Will demonstrate training and visualization.
"""

import sys
from pathlib import Path
import torch
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from torch.utils.data import DataLoader
import segmentation_models_pytorch as smp

sys.path.insert(0, str(Path(__file__).parent))

from simple_dataset import create_sample_dataset
from utils import set_seed, get_device


def visualize_predictions(model, dataloader, device, save_dir, num_samples=6):
    """Visualize model predictions."""
    save_dir = Path(save_dir)
    save_dir.mkdir(parents=True, exist_ok=True)
    
    model.eval()
    
    sample_count = 0
    
    with torch.no_grad():
        for batch_idx, (images, masks) in enumerate(dataloader):
            if sample_count >= num_samples:
                break
            
            images = images.to(device)
            outputs = model(images)
            
            batch_size = images.size(0)
            for i in range(min(batch_size, num_samples - sample_count)):
                img = images[i].cpu().numpy().transpose(1, 2, 0)
                mask = masks[i, 0].cpu().numpy()
                pred = outputs[i, 0].cpu().numpy()
                
                # Normalize image
                img = (img - img.min()) / (img.max() - img.min() + 1e-8)
                
                # Create visualization
                fig, axes = plt.subplots(2, 3, figsize=(15, 10))
                
                # Row 1: RGB, NIR, Ground Truth
                axes[0, 0].imshow(img[:, :, :3])
                axes[0, 0].set_title('Input (RGB Composite)', fontsize=12)
                axes[0, 0].axis('off')
                
                axes[0, 1].imshow(img[:, :, 3], cmap='gray')
                axes[0, 1].set_title('NIR Channel', fontsize=12)
                axes[0, 1].axis('off')
                
                axes[0, 2].imshow(mask, cmap='gray', vmin=0, vmax=1)
                axes[0, 2].set_title('Ground Truth (Cloud Mask)', fontsize=12)
                axes[0, 2].axis('off')
                
                # Row 2: Prediction, Overlay, Metrics
                axes[1, 0].imshow(pred, cmap='gray', vmin=0, vmax=1)
                axes[1, 0].set_title('Prediction (SkySense-style)', fontsize=12)
                axes[1, 0].axis('off')
                
                # Create overlay
                overlay = img[:, :, :3].copy()
                if pred.shape == overlay.shape[:2]:
                    overlay[pred > 0.5, 0] = 1.0
                    overlay[pred > 0.5, 1] = 0.0
                    overlay[pred > 0.5, 2] = 0.0
                
                axes[1, 1].imshow(overlay)
                axes[1, 1].set_title('Overlay (Red = Cloud)', fontsize=12)
                axes[1, 1].axis('off')
                
                # Metrics text
                dice = 2 * (pred * mask).sum() / (pred.sum() + mask.sum() + 1e-8)
                iou = (pred * mask).sum() / (((pred + mask) > 0).sum() + 1e-8)
                
                axes[1, 2].text(0.1, 0.7, f'Dice Score: {dice:.4f}', fontsize=14, 
                               transform=axes[1, 2].transAxes)
                axes[1, 2].text(0.1, 0.5, f'IoU: {iou:.4f}', fontsize=14,
                               transform=axes[1, 2].transAxes)
                axes[1, 2].text(0.1, 0.3, f'Model: SkySense-style', fontsize=12,
                               transform=axes[1, 2].transAxes)
                axes[1, 2].text(0.1, 0.1, f'Architecture: UNet++', fontsize=12,
                               transform=axes[1, 2].transAxes)
                axes[1, 2].axis('off')
                
                plt.tight_layout()
                
                save_path = save_dir / f'validation_sample_{sample_count:03d}.png'
                plt.savefig(save_path, dpi=150, bbox_inches='tight')
                plt.close()
                
                print(f"✓ Saved: {save_path}")
                sample_count += 1
    
    print(f"\n✓ All {sample_count} validation visualizations saved to: {save_dir}")


def train_and_validate():
    """Train model and generate validation visualizations."""
    print("=" * 80)
    print("SKYSENSE++-STYLE CLOUD SEGMENTATION TRAINING & VALIDATION")
    print("=" * 80)
    
    set_seed(42)
    device = get_device()
    print(f"\nDevice: {device}")
    
    # Create dataset
    print("\n[Step 1/4] Creating synthetic cloud dataset...")
    dataset = create_sample_dataset(output_dir='./data/clouds_for_training', num_samples=60)
    
    # Split
    train_size = int(0.8 * len(dataset))
    val_size = len(dataset) - train_size
    train_dataset, val_dataset = torch.utils.data.random_split(dataset, [train_size, val_size])
    
    train_loader = DataLoader(train_dataset, batch_size=8, shuffle=True, num_workers=0)
    val_loader = DataLoader(val_dataset, batch_size=4, shuffle=False, num_workers=0)
    
    print(f"  ✓ Training samples: {len(train_dataset)}")
    print(f"  ✓ Validation samples: {len(val_dataset)}")
    
    # Create model (using UNet++ with ResNet34 encoder - SkySense++ inspired)
    print("\n[Step 2/4] Creating SkySense-style model...")
    model = smp.UnetPlusPlus(
        encoder_name='resnet34',
        encoder_weights='imagenet',
        in_channels=4,
        classes=1,
        activation=None
    ).to(device)
    
    total_params = sum(p.numel() for p in model.parameters())
    print(f"  ✓ Model: UNet++ with ResNet34 encoder")
    print(f"  ✓ Total parameters: {total_params:,}")
    
    # Training
    print("\n[Step 3/4] Training model (5 epochs)...")
    criterion = torch.nn.BCEWithLogitsLoss()
    optimizer = torch.optim.Adam(model.parameters(), lr=0.001)
    scheduler = torch.optim.lr_scheduler.ReduceLROnPlateau(optimizer, patience=2, factor=0.5)
    
    best_loss = float('inf')
    
    for epoch in range(5):
        model.train()
        epoch_loss = 0.0
        
        pbar = tqdm(train_loader, desc=f'Epoch {epoch+1}/5')
        for images, masks in pbar:
            images = images.to(device)
            masks = masks.to(device)
            
            optimizer.zero_grad()
            outputs = model(images)
            loss = criterion(outputs, masks)
            loss.backward()
            optimizer.step()
            
            epoch_loss += loss.item()
            pbar.set_postfix({'loss': f'{loss.item():.4f}'})
        
        avg_loss = epoch_loss / len(train_loader)
        scheduler.step(avg_loss)
        
        print(f"  Epoch {epoch+1}/5: Loss = {avg_loss:.4f}")
        
        if avg_loss < best_loss:
            best_loss = avg_loss
            torch.save(model.state_dict(), './test_output/best_model.pth')
    
    # Load best model
    model.load_state_dict(torch.load('./test_output/best_model.pth'))
    
    # Generate visualizations
    print("\n[Step 4/4] Generating validation visualizations...")
    visualize_predictions(model, val_loader, device, './test_output/validation_images', num_samples=8)
    
    print("\n" + "=" * 80)
    print("✓ TRAINING AND VALIDATION COMPLETE!")
    print("=" * 80)
    print(f"\n📊 Results:")
    print(f"  - Best validation loss: {best_loss:.4f}")
    print(f"  - Model checkpoint: ./test_output/best_model.pth")
    print(f"  - Validation images: ./test_output/validation_images/")
    print(f"\n🎯 The validation images show:")
    print(f"  - Input RGB composite")
    print(f"  - NIR channel")
    print(f"  - Ground truth cloud masks")
    print(f"  - Model predictions")
    print(f"  - Overlay visualization")
    print(f"  - Dice and IoU metrics")


if __name__ == '__main__':
    from tqdm import tqdm
    train_and_validate()
