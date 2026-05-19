#!/usr/bin/env python3
"""
Cloud Segmentation Training Script

Supports multiple datasets and sampling strategies.
All configuration is loaded from config/config.yaml.
"""

import argparse
import sys
from pathlib import Path
from tqdm import tqdm

import torch
import torch.nn as nn
import torch.optim as optim

# Add project root to path
project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

from utils import load_config, set_seed, get_device, save_checkpoint, count_parameters
from models.unet import create_model_from_config
from models.losses import get_loss_function_from_config
from models.metrics import SegmentationMetrics
from data.multi_dataset_loader import get_combined_dataloader, print_dataset_summary


def train(config, strategy="weighted", dataset_names=None, sample_size=None, quick=False):
    """
    Main training function.
    
    Args:
        config: Config object
        strategy: Sampling strategy (concat, weighted, round_robin, balanced, sample)
        dataset_names: List of dataset names to use (default: all available)
        sample_size: Number of samples per dataset for quick validation
        quick: Quick training mode (fewer epochs)
    """
    set_seed(42)
    device = get_device()
    print(f"Using device: {device}")
    
    # Load dataset registry from config
    if dataset_names is None:
        dataset_names = list(config.datasets.keys())
    
    print("\n" + "=" * 80)
    print("CONFIGURATION SUMMARY")
    print("=" * 80)
    print(f"Model: {config.model.get('architecture')} with {config.model.get('encoder_name')}")
    print(f"Dataset strategy: {strategy}")
    print(f"Datasets: {dataset_names}")
    print(f"Batch size: {config.training.get('batch_size')}")
    print(f"Epochs: {config.training.get('epochs') if not quick else 5}")
    print("=" * 80 + "\n")
    
    print_dataset_summary(config)
    
    # Create data loader
    print(f"\nCreating DataLoader with {strategy} strategy...")
    train_loader = get_combined_dataloader(
        dataset_names=dataset_names,
        strategy=strategy,
        batch_size=config.training.get('batch_size', 8),
        shuffle=strategy not in ["weighted", "round_robin"],
        num_workers=config.training.get('num_workers', 0),
        sample_size=sample_size if sample_size else config.sampling.get('sample_size', 100),
        config=config,
        augment=True,
        target_channels=config.data.get('target_channels', 4)
    )
    print(f"Training data loaded: {len(train_loader.dataset)} samples, {len(train_loader)} batches")
    
    # Create model
    print(f"\nCreating model...")
    model = create_model_from_config(config).to(device)
    print(f"Model created: {count_parameters(model):,} trainable parameters")
    
    # Loss function and optimizer
    criterion = get_loss_function_from_config(config)
    optimizer = optim.Adam(
        model.parameters(),
        lr=config.training.get('learning_rate', 0.001),
        weight_decay=config.training.get('weight_decay', 1e-5)
    )
    scheduler = optim.lr_scheduler.ReduceLROnPlateau(
        optimizer,
        mode='min',
        patience=config.training.get('scheduler_patience', 3),
        factor=config.training.get('scheduler_factor', 0.5),
        verbose=True
    )
    
    # Training setup
    num_epochs = config.training.get('epochs', 20) if not quick else 5
    best_loss = float('inf')
    checkpoint_dir = Path(config.training.get('checkpoint_dir', './checkpoints'))
    checkpoint_dir.mkdir(parents=True, exist_ok=True)
    checkpoint_name = f'best_model_{strategy}.pth'
    
    # Metrics
    metrics = SegmentationMetrics()
    
    # Training loop
    print(f"\nStarting training for {num_epochs} epochs...")
    print("=" * 80)
    
    for epoch in range(num_epochs):
        model.train()
        epoch_loss = 0.0
        epoch_metrics = {'dice': 0.0, 'iou': 0.0}
        
        pbar = tqdm(train_loader, desc=f"Epoch {epoch+1}/{num_epochs}", leave=False)
        
        for batch_idx, (images, masks) in enumerate(pbar):
            images = images.to(device)
            masks = masks.to(device)
            
            # Forward pass
            outputs = model(images)
            loss = criterion(outputs, masks)
            
            # Backward pass
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()
            
            epoch_loss += loss.item()
            
            # Calculate metrics
            batch_metrics = metrics(outputs, masks)
            epoch_metrics['dice'] += batch_metrics['dice']
            epoch_metrics['iou'] += batch_metrics['iou']
            
            # Update progress bar
            pbar.set_postfix({
                'loss': f'{loss.item():.4f}',
                'dice': f'{batch_metrics["dice"]:.4f}'
            })
        
        # Average epoch metrics
        avg_loss = epoch_loss / len(train_loader)
        avg_dice = epoch_metrics['dice'] / len(train_loader)
        avg_iou = epoch_metrics['iou'] / len(train_loader)
        
        # Update scheduler
        scheduler.step(avg_loss)
        
        print(f"Epoch {epoch+1}/{num_epochs} - "
              f"Loss: {avg_loss:.4f}, "
              f"Dice: {avg_dice:.4f}, "
              f"IoU: {avg_iou:.4f}")
        
        # Save best model
        if avg_loss < best_loss:
            best_loss = avg_loss
            save_checkpoint(
                model=model,
                optimizer=optimizer,
                epoch=epoch+1,
                loss=best_loss,
                checkpoint_dir=checkpoint_dir,
                filename=checkpoint_name
            )
            print(f"  ✓ Saved best model (loss: {best_loss:.4f})")
    
    print("\n" + "=" * 80)
    print("TRAINING COMPLETED")
    print("=" * 80)
    print(f"Best loss: {best_loss:.4f}")
    print(f"Best model saved to: {checkpoint_dir / checkpoint_name}")
    
    return model, best_loss


def main():
    parser = argparse.ArgumentParser(description='Cloud Segmentation Training')
    parser.add_argument('--config', type=str, default=None,
                        help='Path to config file (default: config/config.yaml)')
    parser.add_argument('--strategy', type=str, default='weighted',
                        choices=['concat', 'weighted', 'round_robin', 'balanced', 'sample'],
                        help='Dataset sampling strategy (default: weighted)')
    parser.add_argument('--datasets', type=str, nargs='+', default=None,
                        help='List of datasets to use (default: all available)')
    parser.add_argument('--sample-size', type=int, default=None,
                        help='Number of samples per dataset for quick validation')
    parser.add_argument('--quick', action='store_true',
                        help='Quick training mode (fewer epochs)')
    
    args = parser.parse_args()
    
    # Load config
    if args.config:
        config = load_config(Path(args.config))
    else:
        config = load_config()
    
    # Run training
    train(
        config=config,
        strategy=args.strategy,
        dataset_names=args.datasets,
        sample_size=args.sample_size,
        quick=args.quick
    )


if __name__ == "__main__":
    main()
