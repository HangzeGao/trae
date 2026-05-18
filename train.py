"""
Training script for UNet cloud segmentation
"""

import argparse
import yaml
import torch
import torch.optim as optim
from pathlib import Path
from tqdm import tqdm
import numpy as np

from models.unet import UNet, UNetWithBackbone
from models.losses import get_loss_function
from models.metrics import SegmentationMetrics


class Trainer:
    """Trainer class for cloud segmentation"""
    
    def __init__(self, config, device):
        self.config = config
        self.device = device
        
        # Model
        model_name = config['model']['name']
        if model_name == 'UNetWithBackbone':
            backbone = config['model'].get('backbone', 'resnet34')
            pretrained = config['model'].get('pretrained', True)
            self.model = UNetWithBackbone(
                backbone_name=backbone,
                num_classes=config['model']['out_channels'],
                pretrained=pretrained
            ).to(device)
        else:
            self.model = UNet(
                in_channels=config['model']['in_channels'],
                out_channels=config['model']['out_channels'],
                init_features=config['model'].get('init_features', 64)
            ).to(device)
        
        # Loss
        self.loss_fn = get_loss_function(config['training']['loss'])
        
        # Optimizer
        lr = config['training']['learning_rate']
        weight_decay = config['training']['weight_decay']
        
        if config['training']['optimizer'].lower() == 'adam':
            self.optimizer = optim.Adam(self.model.parameters(), lr=lr, weight_decay=weight_decay)
        else:
            self.optimizer = optim.SGD(self.model.parameters(), lr=lr, weight_decay=weight_decay, momentum=0.9)
        
        # Scheduler
        self.scheduler = optim.lr_scheduler.CosineAnnealingLR(
            self.optimizer,
            T_max=config['training']['epochs']
        )
        
        # Checkpoint directory
        self.checkpoint_dir = Path(config['training']['checkpoint_dir'])
        self.checkpoint_dir.mkdir(parents=True, exist_ok=True)
        
        # Metrics
        self.best_val_dice = 0
        self.patience = config['training']['patience']
        self.patience_counter = 0
    
    def train_epoch(self, train_loader):
        """Train one epoch"""
        self.model.train()
        total_loss = 0
        
        with tqdm(train_loader, desc="Training") as pbar:
            for images, masks in pbar:
                images = images.to(self.device)
                masks = masks.to(self.device)
                
                # Forward pass
                outputs = self.model(images)
                loss = self.loss_fn(outputs, masks)
                
                # Backward pass
                self.optimizer.zero_grad()
                loss.backward()
                self.optimizer.step()
                
                total_loss += loss.item()
                pbar.set_postfix({'loss': loss.item()})
        
        return total_loss / len(train_loader)
    
    def validate(self, val_loader):
        """Validate"""
        self.model.eval()
        total_dice = 0
        total_iou = 0
        
        with torch.no_grad():
            with tqdm(val_loader, desc="Validation") as pbar:
                for images, masks in pbar:
                    images = images.to(self.device)
                    masks = masks.to(self.device)
                    
                    # Forward pass
                    outputs = self.model(images)
                    
                    # Calculate metrics
                    dice = SegmentationMetrics.dice_score(outputs, masks)
                    iou = SegmentationMetrics.iou_score(outputs, masks)
                    
                    total_dice += dice
                    total_iou += iou
                    
                    pbar.set_postfix({'dice': dice, 'iou': iou})
        
        avg_dice = total_dice / len(val_loader)
        avg_iou = total_iou / len(val_loader)
        
        return avg_dice, avg_iou
    
    def train(self, train_loader, val_loader):
        """Full training loop"""
        
        print(f"\nStarting training on {self.device}")
        print(f"Total parameters: {sum(p.numel() for p in self.model.parameters()):,}\n")
        
        for epoch in range(self.config['training']['epochs']):
            # Train
            train_loss = self.train_epoch(train_loader)
            
            # Validate
            val_dice, val_iou = self.validate(val_loader)
            
            # Update scheduler
            self.scheduler.step()
            
            print(f"\nEpoch {epoch+1}/{self.config['training']['epochs']}")
            print(f"  Train Loss: {train_loss:.4f}")
            print(f"  Val Dice:   {val_dice:.4f}")
            print(f"  Val IoU:    {val_iou:.4f}")
            
            # Save best model
            if val_dice > self.best_val_dice:
                self.best_val_dice = val_dice
                self.patience_counter = 0
                
                model_path = self.checkpoint_dir / f"best_model_{epoch+1:03d}.pth"
                torch.save(self.model.state_dict(), model_path)
                print(f"  ✓ Best model saved: {model_path}")
            else:
                self.patience_counter += 1
            
            # Early stopping
            if self.patience_counter >= self.patience:
                print(f"\n✓ Early stopping at epoch {epoch+1}")
                break
        
        print(f"\n✓ Training complete!")
        print(f"  Best Dice Score: {self.best_val_dice:.4f}")


def main(config_path):
    """Main training function"""
    
    # Load config
    with open(config_path) as f:
        config = yaml.safe_load(f)
    
    # Device
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f"Using device: {device}\n")
    
    # Create trainer (NOTE: Data loading would be implemented here)
    trainer = Trainer(config, device)
    
    print("Configuration loaded:")
    print(yaml.dump(config, default_flow_style=False))
    
    # Note: You would load train and val dataloaders here
    # train_loader = create_train_loader(config)
    # val_loader = create_val_loader(config)
    # trainer.train(train_loader, val_loader)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Train UNet for cloud segmentation')
    parser.add_argument('--config', type=str, default='config/config.yaml',
                       help='Path to config file')
    
    args = parser.parse_args()
    main(args.config)
