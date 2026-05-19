
"""
Kaggle 38cloud 和 95cloud 数据集训练脚本 - 修复版
直接使用有效的样本进行训练
"""

import argparse
import yaml
import torch
import torch.optim as optim
from pathlib import Path
from tqdm import tqdm
import numpy as np
import pandas as pd
from PIL import Image
from torch.utils.data import Dataset, DataLoader

from models.unet import create_model
from models.losses import get_loss_function
from models.metrics import SegmentationMetrics


def find_valid_samples(base_path, max_samples=50):
    """查找有效的样本（GT 不全是 0 的样本）"""
    print(f"正在查找有效样本 (最多 {max_samples} 个)...")
    
    valid_names = []
    gt_dir = base_path / "38-Cloud_training/train_gt"
    
    for gt_file in sorted(gt_dir.glob("*.TIF")):
        gt = np.array(Image.open(gt_file))
        if gt.max() > 0:
            # 提取文件名去掉前缀和后缀
            name = gt_file.stem.replace("gt_", "")
            valid_names.append(name)
            if len(valid_names) >= max_samples:
                break
    
    print(f"找到 {len(valid_names)} 个有效样本")
    return valid_names


class KaggleCloudDataset(Dataset):
    """Kaggle 38cloud 和 95cloud 云分割数据集加载器"""
    
    def __init__(self, valid_names, base_path):
        self.valid_names = valid_names
        self.base_path = Path(base_path)
        self.split_dir = self.base_path / "38-Cloud_training"
        
    def __len__(self):
        return len(self.valid_names)
    
    def __getitem__(self, idx):
        """获取单个样本"""
        name = self.valid_names[idx]
        
        # 加载多光谱波段
        blue_path = self.split_dir / "train_blue" / f"blue_{name}.TIF"
        green_path = self.split_dir / "train_green" / f"green_{name}.TIF"
        red_path = self.split_dir / "train_red" / f"red_{name}.TIF"
        nir_path = self.split_dir / "train_nir" / f"nir_{name}.TIF"
        gt_path = self.split_dir / "train_gt" / f"gt_{name}.TIF"
        
        # 读取图像
        blue = np.array(Image.open(blue_path))
        green = np.array(Image.open(green_path))
        red = np.array(Image.open(red_path))
        nir = np.array(Image.open(nir_path))
        
        # 组合成 4 通道图像 (H, W, 4)
        image = np.stack([red, green, blue, nir], axis=-1)
        
        # 读取掩码
        mask = np.array(Image.open(gt_path))
        mask = (mask > 127).astype(np.float32)  # 0-255 -> 0.0-1.0
        
        # 转换为 tensor 并归一化
        image = torch.from_numpy(image).permute(2, 0, 1).float() / 65535.0  # 16-bit 到 [0,1]
        mask = torch.from_numpy(mask).unsqueeze(0).float()
        
        return image, mask


class Trainer:
    """Trainer class for cloud segmentation"""
    
    def __init__(self, config, device):
        self.config = config
        self.device = device
        
        # Model using segmentation_models_pytorch
        model_config = config['model']
        self.model = create_model(
            architecture=model_config.get('architecture', 'Unet'),
            encoder_name=model_config.get('encoder_name', 'resnet34'),
            encoder_weights=model_config.get('encoder_weights', 'imagenet'),
            in_channels=model_config.get('in_channels', 4),
            classes=model_config.get('out_channels', 1),
            activation=model_config.get('activation', 'sigmoid'),
            **model_config.get('model_kwargs', {})
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
    
    # Base path
    base_path = Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4")
    
    # Find valid samples
    all_valid_names = find_valid_samples(base_path, max_samples=100)
    
    # Split into train and validation
    train_names = all_valid_names[:30]  # 使用30个训练
    val_names = all_valid_names[30:40]  # 使用10个验证
    
    print(f"Train samples: {len(train_names)}")
    print(f"Val samples: {len(val_names)}")
    
    # Create datasets
    train_dataset = KaggleCloudDataset(train_names, base_path)
    val_dataset = KaggleCloudDataset(val_names, base_path)
    
    # Create dataloaders
    batch_size = config['training']['batch_size']
    num_workers = config['training']['num_workers']
    
    train_loader = DataLoader(train_dataset, batch_size=batch_size, shuffle=True, num_workers=num_workers)
    val_loader = DataLoader(val_dataset, batch_size=batch_size, shuffle=False, num_workers=num_workers)
    
    # Create trainer
    trainer = Trainer(config, device)
    
    print("\nConfiguration loaded:")
    print(yaml.dump(config, default_flow_style=False))
    
    # Start training
    trainer.train(train_loader, val_loader)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Train on Kaggle Cloud Datasets')
    parser.add_argument('--config', type=str, default='config/config.yaml',
                       help='Path to config file')
    
    args = parser.parse_args()
    main(args.config)
