#!/usr/bin/env python3
"""
SkySense++ Training Script with Synthetic Data
使用模拟数据验证训练流程
"""

import argparse
import sys
from pathlib import Path
from tqdm import tqdm

import torch
import torch.nn as nn
import torch.optim as optim
from torch.utils.data import Dataset, DataLoader

project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

from models.skysense import SkySensePlusPlus
from models.metrics import SegmentationMetrics


class SyntheticCloudDataset(Dataset):
    """合成云数据集"""
    
    def __init__(self, num_samples=100, img_size=256, num_channels=4):
        self.num_samples = num_samples
        self.img_size = img_size
        self.num_channels = num_channels
    
    def __len__(self):
        return self.num_samples
    
    def __getitem__(self, idx):
        image = torch.randn(self.num_channels, self.img_size, self.img_size)
        mask = (torch.rand(1, self.img_size, self.img_size) > 0.6).float()
        return image, mask


def train_model(model, train_loader, criterion, optimizer, scheduler, device, num_epochs):
    """训练模型"""
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
            torch.save(model.state_dict(), 'skysense_best.pth')
            print(f"  ✓ Saved best model (loss: {best_loss:.4f})")
    
    return best_loss


def test_model(model, test_loader, criterion, device):
    """测试模型"""
    model.eval()
    test_loss = 0.0
    metrics = SegmentationMetrics()
    total_dice = 0.0
    total_iou = 0.0
    
    print("\n" + "=" * 80)
    print("Testing on synthetic test dataset")
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
    parser = argparse.ArgumentParser(description='SkySense++ Training with Synthetic Data')
    parser.add_argument('--epochs', type=int, default=5, help='Number of training epochs')
    parser.add_argument('--batch_size', type=int, default=4, help='Batch size')
    parser.add_argument('--quick', action='store_true', help='Quick mode with fewer samples')
    
    args = parser.parse_args()
    
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f"Using device: {device}")
    
    # 创建合成数据集
    num_train_samples = 50 if args.quick else 200
    num_test_samples = 20 if args.quick else 100
    
    print("\n" + "=" * 80)
    print("SkySense++ CONFIGURATION")
    print("=" * 80)
    print(f"Model: SkySensePlusPlus")
    print(f"  Embed Dim: 128")
    print(f"  Num Heads: 4")
    print(f"  Num Layers: 3")
    print(f"  Patch Size: 16")
    print(f"Training samples: {num_train_samples}")
    print(f"Testing samples: {num_test_samples}")
    print("=" * 80 + "\n")
    
    train_dataset = SyntheticCloudDataset(num_samples=num_train_samples)
    test_dataset = SyntheticCloudDataset(num_samples=num_test_samples)
    
    train_loader = DataLoader(train_dataset, batch_size=args.batch_size, shuffle=True)
    test_loader = DataLoader(test_dataset, batch_size=args.batch_size, shuffle=False)
    
    # 创建模型
    print("Creating SkySense++ model...")
    model = SkySensePlusPlus(
        in_channels=4,
        embed_dim=128,
        num_heads=4,
        num_layers=3,
        patch_size=16,
        out_channels=1
    ).to(device)
    
    total_params = sum(p.numel() for p in model.parameters())
    print(f"Model created: {total_params:,} trainable parameters")
    
    # Loss and optimizer
    criterion = nn.BCELoss()
    optimizer = optim.Adam(model.parameters(), lr=0.001, weight_decay=1e-5)
    scheduler = optim.lr_scheduler.ReduceLROnPlateau(optimizer, mode='min', patience=2, factor=0.5)
    
    # Training
    train_model(model, train_loader, criterion, optimizer, scheduler, device, args.epochs)
    
    # Testing
    test_model(model, test_loader, criterion, device)
    
    print("\n" + "=" * 80)
    print("SkySense++ Training and Testing Completed!")
    print("=" * 80)


if __name__ == "__main__":
    main()