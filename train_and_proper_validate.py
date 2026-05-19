"""
正确的训练和验证脚本
使用独立的验证集，避免全云样本干扰
"""
import argparse
import yaml
import torch
import torch.optim as optim
from pathlib import Path
from tqdm import tqdm
import numpy as np
from PIL import Image
from torch.utils.data import Dataset, DataLoader

from models.unet import create_model
from models.losses import get_loss_function
from models.metrics import SegmentationMetrics


def get_valid_samples(base_path, max_samples=100):
    """获取有效样本，排除全云或无云的极端样本"""
    gt_dir = base_path / "38-Cloud_training/train_gt"
    
    valid_samples = []
    for gt_file in sorted(gt_dir.glob("*.TIF"))[:max_samples*2]:
        gt = np.array(Image.open(gt_file))
        cloud_ratio = (gt > 0).mean()
        
        # 只保留有一定混合的样本：不是全云也不是无云
        if 0.1 < cloud_ratio < 0.9:
            name = gt_file.stem.replace("gt_", "")
            valid_samples.append((name, cloud_ratio))
            if len(valid_samples) >= max_samples:
                break
    
    return valid_samples


class KaggleCloudDataset(Dataset):
    def __init__(self, names, base_path):
        self.names = names
        self.base_path = Path(base_path)
        self.split_dir = self.base_path / "38-Cloud_training"
    
    def __len__(self):
        return len(self.names)
    
    def __getitem__(self, idx):
        name = self.names[idx]
        
        blue = np.array(Image.open(self.split_dir / "train_blue" / f"blue_{name}.TIF"))
        green = np.array(Image.open(self.split_dir / "train_green" / f"green_{name}.TIF"))
        red = np.array(Image.open(self.split_dir / "train_red" / f"red_{name}.TIF"))
        nir = np.array(Image.open(self.split_dir / "train_nir" / f"nir_{name}.TIF"))
        gt = np.array(Image.open(self.split_dir / "train_gt" / f"gt_{name}.TIF"))
        
        image = np.stack([red, green, blue, nir], axis=-1)
        mask = (gt > 127).astype(np.float32)
        
        image = torch.from_numpy(image).permute(2, 0, 1).float() / 65535.0
        mask = torch.from_numpy(mask).unsqueeze(0).float()
        
        return image, mask


class Trainer:
    def __init__(self, config, device):
        self.config = config
        self.device = device
        
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
        
        self.loss_fn = get_loss_function(config['training']['loss'])
        
        lr = config['training']['learning_rate']
        weight_decay = config['training']['weight_decay']
        if config['training']['optimizer'].lower() == 'adam':
            self.optimizer = optim.Adam(self.model.parameters(), lr=lr, weight_decay=weight_decay)
        else:
            self.optimizer = optim.SGD(self.model.parameters(), lr=lr, weight_decay=weight_decay, momentum=0.9)
        
        self.scheduler = optim.lr_scheduler.CosineAnnealingLR(
            self.optimizer, T_max=config['training']['epochs']
        )
        
        self.checkpoint_dir = Path(config['training']['checkpoint_dir'])
        self.checkpoint_dir.mkdir(parents=True, exist_ok=True)
        
        self.best_val_dice = 0
    
    def train_epoch(self, train_loader):
        self.model.train()
        total_loss = 0
        for images, masks in tqdm(train_loader, desc="Training"):
            images = images.to(self.device)
            masks = masks.to(self.device)
            
            outputs = self.model(images)
            loss = self.loss_fn(outputs, masks)
            
            self.optimizer.zero_grad()
            loss.backward()
            self.optimizer.step()
            
            total_loss += loss.item()
        return total_loss / len(train_loader)
    
    def validate(self, val_loader):
        self.model.eval()
        total_dice = 0
        total_iou = 0
        
        with torch.no_grad():
            for images, masks in tqdm(val_loader, desc="Validating"):
                images = images.to(self.device)
                masks = masks.to(self.device)
                outputs = self.model(images)
                
                dice = SegmentationMetrics.dice_score(outputs, masks)
                iou = SegmentationMetrics.iou_score(outputs, masks)
                
                total_dice += dice
                total_iou += iou
        
        avg_dice = total_dice / len(val_loader)
        avg_iou = total_iou / len(val_loader)
        return avg_dice, avg_iou
    
    def train(self, train_loader, val_loader):
        print(f"\nStarting proper training!")
        print(f"  Train samples: {len(train_loader.dataset)}")
        print(f"  Val samples: {len(val_loader.dataset)}")
        print(f"  Total params: {sum(p.numel() for p in self.model.parameters()):,}\n")
        
        for epoch in range(self.config['training']['epochs']):
            train_loss = self.train_epoch(train_loader)
            val_dice, val_iou = self.validate(val_loader)
            self.scheduler.step()
            
            print(f"\nEpoch {epoch+1}/{self.config['training']['epochs']}")
            print(f"  Train Loss: {train_loss:.4f}")
            print(f"  Val Dice: {val_dice:.4f}")
            print(f"  Val IoU:  {val_iou:.4f}")
            
            if val_dice > self.best_val_dice:
                self.best_val_dice = val_dice
                model_path = self.checkpoint_dir / "proper_best_model.pth"
                torch.save(self.model.state_dict(), model_path)
                print(f"  ✓ Best model saved!")
        
        print(f"\n✓ Training complete!")
        print(f"  Best Val Dice: {self.best_val_dice:.4f}")


def main(config_path):
    with open(config_path) as f:
        config = yaml.safe_load(f)
    
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f"Using device: {device}\n")
    
    base_path = Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4")
    
    # 获取有效样本，排除极端情况
    print("Collecting valid samples (10-90% cloud cover)...")
    valid_samples = get_valid_samples(base_path, max_samples=60)
    print(f"Found {len(valid_samples)} suitable samples")
    
    # 正确划分：打乱，然后 80% 训练，20% 验证
    np.random.seed(42)
    np.random.shuffle(valid_samples)
    
    split_idx = int(0.8 * len(valid_samples))
    train_names = [s[0] for s in valid_samples[:split_idx]]
    val_names = [s[0] for s in valid_samples[split_idx:]]
    
    print(f"\nData split:")
    print(f"  Train: {len(train_names)} samples")
    print(f"  Val:   {len(val_names)} samples")
    
    # 创建数据集
    train_dataset = KaggleCloudDataset(train_names, base_path)
    val_dataset = KaggleCloudDataset(val_names, base_path)
    
    batch_size = config['training']['batch_size']
    train_loader = DataLoader(train_dataset, batch_size=batch_size, shuffle=True, num_workers=0)
    val_loader = DataLoader(val_dataset, batch_size=batch_size, shuffle=False, num_workers=0)
    
    # 训练
    trainer = Trainer(config, device)
    trainer.train(train_loader, val_loader)
    
    # 单独验证并保存结果
    print("\n=== Final Validation ===")
    trainer.model.load_state_dict(torch.load(trainer.checkpoint_dir / "proper_best_model.pth"))
    final_dice, final_iou = trainer.validate(val_loader)
    print(f"\nFinal Results (on independent val set):")
    print(f"  Dice: {final_dice:.4f}")
    print(f"  IoU:  {final_iou:.4f}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('--config', type=str, default='config/config.yaml')
    args = parser.parse_args()
    main(args.config)
