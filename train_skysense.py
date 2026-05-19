#!/usr/bin/env python3
"""
SkySense++ Training Script
Training on 38-Cloud dataset, testing on 95-Cloud dataset.
"""

import argparse
import sys
from pathlib import Path
from tqdm import tqdm

import torch
import torch.nn as nn
import torch.optim as optim

project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

from utils import load_config, set_seed, get_device, save_checkpoint, count_parameters
from models.unet import create_model_from_config
from models.losses import get_loss_function_from_config
from models.metrics import SegmentationMetrics
from data.multi_dataset_loader import get_combined_dataloader, print_dataset_summary


def train_model(model, train_loader, criterion, optimizer, scheduler, device, num_epochs, checkpoint_dir, config):
    """
    Train the model.
    
    Args:
        model: PyTorch model
        train_loader: Training data loader
        criterion: Loss function
        optimizer: Optimizer
        scheduler: Learning rate scheduler
        device: Device to train on
        num_epochs: Number of epochs
        checkpoint_dir: Directory to save checkpoints
        config: Config object
    
    Returns:
        best_loss: Best validation loss
    """
    best_loss = float('inf')
    metrics = SegmentationMetrics()
    
    print(f"\nStarting training for {num_epochs} epochs...")
    print("=" * 80)
    
    for epoch in range(num_epochs):
        model.train()
        epoch_loss = 0.0
        epoch_dice = 0.0
        
        pbar = tqdm(train_loader, desc=f"Epoch {epoch+1}/{num_epochs}", leave=False)
        
        for images, masks in pbar:
            images = images.to(device)
            masks = masks.to(device)
            
            outputs = model(images)
            loss = criterion(outputs, masks)
            
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()
            
            epoch_loss += loss.item()
            epoch_dice += metrics(outputs, masks)['dice']
            
            pbar.set_postfix({
                'loss': f'{loss.item():.4f}',
                'dice': f'{metrics(outputs, masks)["dice"]:.4f}'
            })
        
        avg_loss = epoch_loss / len(train_loader)
        avg_dice = epoch_dice / len(train_loader)
        
        scheduler.step(avg_loss)
        
        print(f"Epoch {epoch+1}/{num_epochs} - "
              f"Loss: {avg_loss:.4f}, "
              f"Dice: {avg_dice:.4f}")
        
        if avg_loss < best_loss:
            best_loss = avg_loss
            save_checkpoint(
                model=model,
                optimizer=optimizer,
                epoch=epoch+1,
                loss=best_loss,
                checkpoint_dir=checkpoint_dir,
                filename='skysense_best.pth'
            )
            print(f"  ✓ Saved best model (loss: {best_loss:.4f})")
    
    return best_loss


def test_model(model, test_loader, criterion, device):
    """
    Test the model on test dataset.
    
    Args:
        model: PyTorch model
        test_loader: Test data loader
        criterion: Loss function
        device: Device to test on
    
    Returns:
        dict: Test metrics
    """
    model.eval()
    test_loss = 0.0
    metrics = SegmentationMetrics()
    total_dice = 0.0
    total_iou = 0.0
    
    print("\n" + "=" * 80)
    print("Testing on 95-Cloud dataset")
    print("=" * 80)
    
    with torch.no_grad():
        pbar = tqdm(test_loader, desc="Testing", leave=False)
        for images, masks in pbar:
            images = images.to(device)
            masks = masks.to(device)
            
            outputs = model(images)
            loss = criterion(outputs, masks)
            
            test_loss += loss.item()
            batch_metrics = metrics(outputs, masks)
            total_dice += batch_metrics['dice']
            total_iou += batch_metrics['iou']
    
    avg_loss = test_loss / len(test_loader)
    avg_dice = total_dice / len(test_loader)
    avg_iou = total_iou / len(test_loader)
    
    print(f"\nTest Results:")
    print(f"  Loss: {avg_loss:.4f}")
    print(f"  Dice Coefficient: {avg_dice:.4f}")
    print(f"  IoU: {avg_iou:.4f}")
    
    return {
        'loss': avg_loss,
        'dice': avg_dice,
        'iou': avg_iou
    }


def main():
    parser = argparse.ArgumentParser(description='SkySense++ Training and Testing')
    parser.add_argument('--config', type=str, default=None,
                        help='Path to config file')
    parser.add_argument('--quick', action='store_true',
                        help='Quick training mode (fewer epochs)')
    parser.add_argument('--epochs', type=int, default=None,
                        help='Number of training epochs')
    
    args = parser.parse_args()
    
    set_seed(42)
    device = get_device()
    print(f"Using device: {device}")
    
    # Load config
    if args.config:
        config = load_config(Path(args.config))
    else:
        config = load_config()
    
    # Update config for SkySense++
    config.model['architecture'] = 'SkySensePlusPlus'
    config.model['in_channels'] = 4
    config.model['out_channels'] = 1
    config.model['embed_dim'] = 128
    config.model['num_heads'] = 4
    config.model['num_layers'] = 3
    config.model['patch_size'] = 16
    
    print("\n" + "=" * 80)
    print("SkySense++ CONFIGURATION")
    print("=" * 80)
    print(f"Model: {config.model.get('architecture')}")
    print(f"  Embed Dim: {config.model.get('embed_dim')}")
    print(f"  Num Heads: {config.model.get('num_heads')}")
    print(f"  Num Layers: {config.model.get('num_layers')}")
    print(f"  Patch Size: {config.model.get('patch_size')}")
    print(f"Training on: 38-Cloud")
    print(f"Testing on: 95-Cloud")
    print("=" * 80 + "\n")
    
    # Training data (38cloud)
    print("Loading training data (38-Cloud)...")
    train_loader = get_combined_dataloader(
        dataset_names=['38cloud'],
        strategy='sample',
        batch_size=config.training.get('batch_size', 8),
        shuffle=True,
        num_workers=config.training.get('num_workers', 0),
        sample_size=100,
        config=config,
        augment=True,
        target_channels=4
    )
    print(f"Training samples: {len(train_loader.dataset)}")
    
    # Testing data (95cloud)
    print("\nLoading testing data (95-Cloud)...")
    test_loader = get_combined_dataloader(
        dataset_names=['95cloud'],
        strategy='sample',
        batch_size=config.training.get('batch_size', 8),
        shuffle=False,
        num_workers=config.training.get('num_workers', 0),
        sample_size=50,
        config=config,
        augment=False,
        target_channels=4
    )
    print(f"Testing samples: {len(test_loader.dataset)}")
    
    # Create model
    print("\nCreating SkySense++ model...")
    model = create_model_from_config(config).to(device)
    print(f"Model created: {count_parameters(model):,} trainable parameters")
    
    # Loss and optimizer
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
        factor=config.training.get('scheduler_factor', 0.5)
    )
    
    # Training
    num_epochs = args.epochs if args.epochs else (5 if args.quick else config.training.get('epochs', 20))
    checkpoint_dir = Path(config.training.get('checkpoint_dir', './checkpoints'))
    checkpoint_dir.mkdir(parents=True, exist_ok=True)
    
    train_model(model, train_loader, criterion, optimizer, scheduler, device, num_epochs, checkpoint_dir, config)
    
    # Testing
    test_model(model, test_loader, criterion, device)
    
    print("\n" + "=" * 80)
    print("SkySense++ Training and Testing Completed!")
    print("=" * 80)


if __name__ == "__main__":
    main()