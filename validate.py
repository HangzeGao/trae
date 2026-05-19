#!/usr/bin/env python3
"""
Cloud Segmentation Validation Script

Validates trained model on multiple datasets.
All configuration is loaded from config/config.yaml.
"""

import argparse
import sys
from pathlib import Path
from tqdm import tqdm

import torch

# Add project root to path
project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

from utils import load_config, get_device, load_checkpoint, count_parameters
from models.unet import create_model_from_config
from models.metrics import SegmentationMetrics
from data.multi_dataset_loader import get_combined_dataloader, print_dataset_summary, list_available_datasets_with_stats


def validate(config, checkpoint_path, dataset_names=None, strategy="balanced", sample_size=None):
    """
    Validate a trained model.
    
    Args:
        config: Config object
        checkpoint_path: Path to model checkpoint
        dataset_names: List of datasets to validate on (default: all available)
        strategy: Sampling strategy for validation
        sample_size: Number of samples per dataset
    """
    device = get_device()
    print(f"Using device: {device}")
    
    # Load dataset registry from config
    if dataset_names is None:
        dataset_names = list(config.datasets.keys())
    
    print("\n" + "=" * 80)
    print("VALIDATION CONFIGURATION")
    print("=" * 80)
    print(f"Model: {config.model.get('architecture')} with {config.model.get('encoder_name')}")
    print(f"Checkpoint: {checkpoint_path}")
    print(f"Datasets: {dataset_names}")
    print("=" * 80 + "\n")
    
    print_dataset_summary(config)
    
    # Create data loader
    print(f"\nCreating DataLoader with {strategy} strategy...")
    val_loader = get_combined_dataloader(
        dataset_names=dataset_names,
        strategy=strategy,
        batch_size=config.training.get('batch_size', 8),
        shuffle=False,
        num_workers=config.training.get('num_workers', 0),
        sample_size=sample_size if sample_size else config.sampling.get('sample_size', 100),
        config=config,
        augment=False,
        target_channels=config.data.get('target_channels', 4)
    )
    print(f"Validation data loaded: {len(val_loader.dataset)} samples, {len(val_loader)} batches")
    
    # Create and load model
    print(f"\nCreating model...")
    model = create_model_from_config(config).to(device)
    print(f"Model created: {count_parameters(model):,} parameters")
    
    checkpoint = load_checkpoint(model, checkpoint_path, device=device)
    print(f"Loaded checkpoint from epoch {checkpoint.get('epoch', 'unknown')} with loss {checkpoint.get('loss', 'unknown'):.4f}")
    model.eval()
    
    # Metrics
    metrics = SegmentationMetrics()
    total_metrics = {'dice': 0.0, 'iou': 0.0}
    
    # Validation loop
    print(f"\nStarting validation...")
    print("=" * 80)
    
    with torch.no_grad():
        pbar = tqdm(val_loader, desc="Validating", leave=False)
        
        for images, masks in pbar:
            images = images.to(device)
            masks = masks.to(device)
            
            # Forward pass
            outputs = model(images)
            
            # Calculate metrics
            batch_metrics = metrics(outputs, masks)
            total_metrics['dice'] += batch_metrics['dice']
            total_metrics['iou'] += batch_metrics['iou']
            
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
    print("=" * 80)
    
    return {'dice': avg_dice, 'iou': avg_iou}


def main():
    parser = argparse.ArgumentParser(description='Cloud Segmentation Validation')
    parser.add_argument('--config', type=str, default=None,
                        help='Path to config file (default: config/config.yaml)')
    parser.add_argument('--checkpoint', type=str, required=True,
                        help='Path to model checkpoint file')
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
    validate(
        config=config,
        checkpoint_path=Path(args.checkpoint),
        dataset_names=args.datasets,
        strategy=args.strategy,
        sample_size=args.sample_size
    )


if __name__ == "__main__":
    main()
