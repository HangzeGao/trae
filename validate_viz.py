#!/usr/bin/env python3
"""
Cloud Segmentation Validation with Visualization Script

Validates trained model and generates visualization outputs.
"""

import argparse
import sys
from pathlib import Path
from tqdm import tqdm

import torch
import numpy as np
import matplotlib.pyplot as plt
from PIL import Image

# Add project root to path
project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

from utils import load_config, get_device, load_checkpoint
from models.unet import create_model_from_config
from models.metrics import SegmentationMetrics
from data.multi_dataset_loader import get_combined_dataloader


def visualize_results(images, masks, outputs, save_dir, batch_idx, num_samples=4):
    """
    Generate and save visualization of results.
    
    Args:
        images: Input images tensor
        masks: Ground truth masks
        outputs: Model predictions
        save_dir: Directory to save images
        batch_idx: Batch index for unique filenames
        num_samples: Number of samples to visualize per batch
    """
    save_dir = Path(save_dir)
    save_dir.mkdir(parents=True, exist_ok=True)
    
    # Convert tensors to numpy
    images_np = images.cpu().numpy()
    masks_np = masks.cpu().numpy()
    outputs_np = outputs.cpu().numpy()
    
    num_samples = min(num_samples, images.shape[0])
    
    for sample_idx in range(num_samples):
        # Get RGB image (first 3 channels)
        img = images_np[sample_idx, :3, :, :]  # Take first 3 channels (RGB)
        img = np.transpose(img, (1, 2, 0))
        # Normalize for display
        img = (img - img.min()) / (img.max() - img.min() + 1e-8)
        
        # Get ground truth mask
        mask = masks_np[sample_idx, 0, :, :]
        
        # Get prediction
        pred = outputs_np[sample_idx, 0, :, :]
        pred_binary = (pred > 0.5).astype(np.float32)
        
        # Create figure
        fig, axes = plt.subplots(1, 4, figsize=(16, 4))
        
        # Plot RGB image
        axes[0].imshow(img)
        axes[0].set_title('Input Image (RGB)')
        axes[0].axis('off')
        
        # Plot NIR if available
        if images_np.shape[1] >= 4:
            nir = images_np[sample_idx, 3, :, :]
            nir_img = np.stack([nir, nir, nir], axis=-1)
            nir_img = (nir_img - nir_img.min()) / (nir_img.max() - nir_img.min() + 1e-8)
            axes[1].imshow(nir_img)
            axes[1].set_title('NIR Channel')
            axes[1].axis('off')
        else:
            axes[1].axis('off')
        
        # Plot ground truth
        axes[2].imshow(mask, cmap='gray', vmin=0, vmax=1)
        axes[2].set_title('Ground Truth')
        axes[2].axis('off')
        
        # Plot prediction
        axes[3].imshow(pred_binary, cmap='gray', vmin=0, vmax=1)
        axes[3].set_title('Prediction')
        axes[3].axis('off')
        
        plt.tight_layout()
        plt.savefig(save_dir / f'viz_batch_{batch_idx}_sample_{sample_idx}.png', dpi=150, bbox_inches='tight')
        plt.close()
        
        # Also save individual mask overlays
        fig, ax = plt.subplots(1, 1, figsize=(8, 8))
        ax.imshow(img)
        ax.imshow(pred_binary, cmap='jet', alpha=0.5, vmin=0, vmax=1)
        ax.set_title('Prediction Overlay')
        ax.axis('off')
        plt.savefig(save_dir / f'overlay_batch_{batch_idx}_sample_{sample_idx}.png', dpi=150, bbox_inches='tight')
        plt.close()


def validate_with_visualization(
    config, 
    checkpoint_path, 
    dataset_names=None, 
    strategy="balanced", 
    sample_size=None,
    save_dir="./validation_images"
):
    """
    Validate a trained model and save visualizations.
    
    Args:
        config: Config object
        checkpoint_path: Path to model checkpoint
        dataset_names: List of datasets to validate on
        strategy: Sampling strategy
        sample_size: Number of samples per dataset
        save_dir: Directory to save visualization images
    """
    device = get_device()
    print(f"Using device: {device}")
    
    # Load dataset registry from config
    if dataset_names is None:
        dataset_names = list(config.datasets.keys())
    
    print("\n" + "=" * 80)
    print("VALIDATION WITH VISUALIZATION")
    print("=" * 80)
    print(f"Model: {config.model.get('architecture')} with {config.model.get('encoder_name')}")
    print(f"Checkpoint: {checkpoint_path}")
    print(f"Datasets: {dataset_names}")
    print(f"Save visualization to: {save_dir}")
    print("=" * 80 + "\n")
    
    # Create data loader
    print(f"\nCreating DataLoader with {strategy} strategy...")
    val_loader = get_combined_dataloader(
        dataset_names=dataset_names,
        strategy=strategy,
        batch_size=config.training.get('batch_size', 4),
        shuffle=False,
        num_workers=0,
        sample_size=sample_size if sample_size else config.sampling.get('sample_size', 100),
        config=config,
        augment=False,
        target_channels=config.data.get('target_channels', 4)
    )
    print(f"Validation data loaded: {len(val_loader.dataset)} samples, {len(val_loader)} batches")
    
    # Create and load model
    print(f"\nCreating model...")
    model = create_model_from_config(config).to(device)
    
    checkpoint = load_checkpoint(model, checkpoint_path, device=device)
    print(f"Loaded checkpoint from epoch {checkpoint.get('epoch', 'unknown')} with loss {checkpoint.get('loss', 'unknown'):.4f}")
    model.eval()
    
    # Metrics
    metrics = SegmentationMetrics()
    total_metrics = {'dice': 0.0, 'iou': 0.0}
    
    # Create save directory
    save_path = Path(save_dir)
    save_path.mkdir(parents=True, exist_ok=True)
    print(f"\nVisualization images will be saved to: {save_path.absolute()}")
    
    # Validation loop
    print(f"\nStarting validation...")
    print("=" * 80)
    
    with torch.no_grad():
        pbar = tqdm(val_loader, desc="Validating", leave=False)
        
        for batch_idx, (images, masks) in enumerate(pbar):
            images = images.to(device)
            masks = masks.to(device)
            
            # Forward pass
            outputs = model(images)
            
            # Calculate metrics
            batch_metrics = metrics(outputs, masks)
            total_metrics['dice'] += batch_metrics['dice']
            total_metrics['iou'] += batch_metrics['iou']
            
            # Save visualization every N batches
            if batch_idx % 5 == 0:  # Visualize every 5th batch
                visualize_results(images, masks, outputs, save_path, batch_idx, num_samples=3)
            
            # Update progress bar
            pbar.set_postfix({
                'dice': f'{batch_metrics["dice"]:.4f}',
                'iou': f'{batch_metrics["iou"]:.4f}'
            })
    
    # Average metrics
    avg_dice = total_metrics['dice'] / len(val_loader)
    avg_iou = total_metrics['iou'] / len(val_loader)
    
    print("\n" + "=" * 80)
    print("VALIDATION RESULTS")
    print("=" * 80)
    print(f"Average Dice Coefficient: {avg_dice:.4f}")
    print(f"Average IoU: {avg_iou:.4f}")
    print(f"\nVisualization images saved to: {save_path.absolute()}")
    print("=" * 80)
    
    # Save summary statistics
    with open(save_path / 'validation_summary.txt', 'w') as f:
        f.write(f"Validation Summary\n")
        f.write(f"{'='*80}\n")
        f.write(f"Model: {config.model.get('architecture')}\n")
        f.write(f"Encoder: {config.model.get('encoder_name')}\n")
        f.write(f"Checkpoint: {checkpoint_path}\n")
        f.write(f"Datasets: {dataset_names}\n")
        f.write(f"Average Dice: {avg_dice:.4f}\n")
        f.write(f"Average IoU: {avg_iou:.4f}\n")
    
    return {'dice': avg_dice, 'iou': avg_iou}


def main():
    parser = argparse.ArgumentParser(description='Cloud Segmentation Validation with Visualization')
    parser.add_argument('--config', type=str, default=None,
                        help='Path to config file (default: config/config.yaml)')
    parser.add_argument('--checkpoint', type=str, required=True,
                        help='Path to model checkpoint file')
    parser.add_argument('--save-dir', type=str, default='./validation_images',
                        help='Directory to save visualization images (default: ./validation_images)')
    parser.add_argument('--strategy', type=str, default='balanced',
                        choices=['concat', 'weighted', 'round_robin', 'balanced', 'sample'],
                        help='Dataset sampling strategy (default: balanced)')
    parser.add_argument('--datasets', type=str, nargs='+', default=None,
                        help='List of datasets to use (default: all available)')
    parser.add_argument('--sample-size', type=int, default=None,
                        help='Number of samples per dataset (default: from config)')
    
    args = parser.parse_args()
    
    # Load config
    if args.config:
        config = load_config(Path(args.config))
    else:
        config = load_config()
    
    # Run validation
    validate_with_visualization(
        config=config,
        checkpoint_path=Path(args.checkpoint),
        dataset_names=args.datasets,
        strategy=args.strategy,
        sample_size=args.sample_size,
        save_dir=args.save_dir
    )


if __name__ == "__main__":
    main()
