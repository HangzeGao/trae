"""
统一的 Kaggle Cloud Segmentation 训练和验证脚本
包含数据加载、训练、验证、推理等完整流程
"""
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


class CloudDataset(Dataset):
    """Kaggle 38cloud 云分割数据集"""
    
    def __init__(self, sample_names, base_path):
        self.names = sample_names
        self.base_path = Path(base_path)
        self.split_dir = self.base_path / "38-Cloud_training"
    
    def __len__(self):
        return len(self.names)
    
    def __getitem__(self, idx):
        name = self.names[idx]
        
        # 加载多光谱波段
        blue = np.array(Image.open(self.split_dir / f"train_blue/blue_{name}.TIF"))
        green = np.array(Image.open(self.split_dir / f"train_green/green_{name}.TIF"))
        red = np.array(Image.open(self.split_dir / f"train_red/red_{name}.TIF"))
        nir = np.array(Image.open(self.split_dir / f"train_nir/nir_{name}.TIF"))
        gt = np.array(Image.open(self.split_dir / f"train_gt/gt_{name}.TIF"))
        
        # 组合和归一化
        image = np.stack([red, green, blue, nir], axis=-1)
        image_tensor = torch.from_numpy(image).permute(2, 0, 1).float() / 65535.0
        
        mask = (gt > 127).astype(np.float32)
        mask_tensor = torch.from_numpy(mask).unsqueeze(0).float()
        
        return image_tensor, mask_tensor


def get_valid_samples(base_path, min_ratio=0.1, max_ratio=0.9, max_samples=300):
    """获取有效样本，排除极端情况"""
    gt_dir = base_path / "38-Cloud_training/train_gt"
    valid_samples = []
    
    for gt_file in sorted(gt_dir.glob("*.TIF")):
        gt = np.array(Image.open(gt_file))
        cloud_ratio = (gt > 0).mean()
        
        if min_ratio < cloud_ratio < max_ratio:
            name = gt_file.stem.replace("gt_", "")
            valid_samples.append(name)
            if len(valid_samples) >= max_samples:
                break
    
    return valid_samples


class CloudSegTrainer:
    """云分割训练器"""
    
    def __init__(self, config, device):
        self.config = config
        self.device = device
        
        # 模型
        model_cfg = config['model']
        self.model = create_model(
            architecture=model_cfg['architecture'],
            encoder_name=model_cfg['encoder_name'],
            encoder_weights=model_cfg['encoder_weights'],
            in_channels=model_cfg['in_channels'],
            classes=model_cfg['out_channels'],
            activation=model_cfg['activation'],
            **model_cfg.get('model_kwargs', {})
        ).to(device)
        
        # 损失函数和优化器
        self.loss_fn = get_loss_function(config['training']['loss'])
        lr = config['training']['learning_rate']
        wd = config['training']['weight_decay']
        
        if config['training']['optimizer'].lower() == 'adam':
            self.optimizer = optim.Adam(self.model.parameters(), lr=lr, weight_decay=wd)
        else:
            self.optimizer = optim.SGD(self.model.parameters(), lr=lr, weight_decay=wd, momentum=0.9)
        
        self.scheduler = optim.lr_scheduler.CosineAnnealingLR(
            self.optimizer, T_max=config['training']['epochs']
        )
        
        self.checkpoint_dir = Path(config['training']['checkpoint_dir'])
        self.checkpoint_dir.mkdir(parents=True, exist_ok=True)
        self.best_val_dice = 0
        self.patience_counter = 0
    
    def train_epoch(self, train_loader):
        """训练一个 epoch"""
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
        """验证"""
        self.model.eval()
        total_dice = 0
        total_iou = 0
        
        with torch.no_grad():
            for images, masks in tqdm(val_loader, desc="Validating"):
                images = images.to(self.device)
                masks = masks.to(self.device)
                outputs = self.model(images)
                
                total_dice += SegmentationMetrics.dice_score(outputs, masks)
                total_iou += SegmentationMetrics.iou_score(outputs, masks)
        
        return total_dice/len(val_loader), total_iou/len(val_loader)
    
    def train(self, train_loader, val_loader):
        """完整训练流程"""
        print(f"\n{'='*50}")
        print(f"Cloud Segmentation Training")
        print(f"{'='*50}")
        print(f"Train samples: {len(train_loader.dataset)}")
        print(f"Val samples:   {len(val_loader.dataset)}")
        print(f"Total params:  {sum(p.numel() for p in self.model.parameters()):,}")
        print(f"{'='*50}\n")
        
        for epoch in range(self.config['training']['epochs']):
            train_loss = self.train_epoch(train_loader)
            val_dice, val_iou = self.validate(val_loader)
            self.scheduler.step()
            
            print(f"\nEpoch {epoch+1}/{self.config['training']['epochs']}")
            print(f"  Loss: {train_loss:.4f} | Dice: {val_dice:.4f} | IoU: {val_iou:.4f}")
            
            # 保存最佳模型
            if val_dice > self.best_val_dice:
                self.best_val_dice = val_dice
                self.patience_counter = 0
                
                model_path = self.checkpoint_dir / "best_model.pth"
                torch.save(self.model.state_dict(), model_path)
                print(f"  ✓ New best model saved!")
            else:
                self.patience_counter += 1
            
            # 早停
            if self.patience_counter >= self.config['training']['patience']:
                print(f"\n✓ Early stopping triggered!")
                break
        
        print(f"\n{'='*50}")
        print(f"Training complete! Best Dice: {self.best_val_dice:.4f}")
        print(f"{'='*50}")
        
        return self.best_val_dice


def main():
    # 加载配置
    with open('config/config.yaml') as f:
        config = yaml.safe_load(f)
    
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f"Using device: {device}")
    
    # 获取有效样本
    data_cfg = config['data']
    base_path = Path(data_cfg['base_path'])
    valid_samples = get_valid_samples(
        base_path,
        min_ratio=data_cfg['min_cloud_ratio'],
        max_ratio=data_cfg['max_cloud_ratio'],
        max_samples=300
    )
    
    print(f"\nFound {len(valid_samples)} valid samples (cloud ratio: {data_cfg['min_cloud_ratio']}-{data_cfg['max_cloud_ratio']})")
    
    # 划分训练验证
    np.random.seed(42)
    np.random.shuffle(valid_samples)
    split = int(data_cfg['train_val_split'] * len(valid_samples))
    
    train_names = valid_samples[:split]
    val_names = valid_samples[split:]
    
    print(f"Train: {len(train_names)}, Val: {len(val_names)}\n")
    
    # 数据加载
    train_dataset = CloudDataset(train_names, base_path)
    val_dataset = CloudDataset(val_names, base_path)
    
    train_loader = DataLoader(
        train_dataset,
        batch_size=config['training']['batch_size'],
        shuffle=True,
        num_workers=config['training']['num_workers']
    )
    val_loader = DataLoader(
        val_dataset,
        batch_size=config['training']['batch_size'],
        shuffle=False,
        num_workers=config['training']['num_workers']
    )
    
    # 训练
    trainer = CloudSegTrainer(config, device)
    trainer.train(train_loader, val_loader)
    
    # 最终验证
    print("\n\n=== Final Validation ===")
    trainer.model.load_state_dict(torch.load(trainer.checkpoint_dir / "best_model.pth"))
    final_dice, final_iou = trainer.validate(val_loader)
    
    print(f"\nFinal Results:")
    print(f"  Dice Score: {final_dice:.4f}")
    print(f"  IoU Score:  {final_iou:.4f}")
    print(f"\n✓ All done!")


if __name__ == "__main__":
    main()
