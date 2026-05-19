#!/usr/bin/env python3
"""
轻量级SkySense++ - 无需下载预训练权重
"""

import sys
import numpy as np
import matplotlib.pyplot as plt
from pathlib import Path
from tqdm import tqdm

import torch
import torch.nn as nn
import torch.optim as optim
from torch.utils.data import Dataset, DataLoader

project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))


class SimpleCNN(nn.Module):
    """轻量级CNN模型用于演示"""
    
    def __init__(self, in_channels=4):
        super().__init__()
        
        # 编码器
        self.encoder = nn.Sequential(
            nn.Conv2d(in_channels, 32, 3, padding=1),
            nn.ReLU(inplace=True),
            nn.BatchNorm2d(32),
            nn.Conv2d(32, 64, 3, stride=2, padding=1),
            nn.ReLU(inplace=True),
            nn.BatchNorm2d(64),
            
            nn.Conv2d(64, 128, 3, padding=1),
            nn.ReLU(inplace=True),
            nn.BatchNorm2d(128),
            nn.Conv2d(128, 128, 3, stride=2, padding=1),
            nn.ReLU(inplace=True),
            nn.BatchNorm2d(128),
            
            nn.Conv2d(128, 256, 3, padding=1),
            nn.ReLU(inplace=True),
            nn.BatchNorm2d(256),
        )
        
        # 注意力模块
        self.channel_attention = nn.Sequential(
            nn.AdaptiveAvgPool2d(1),
            nn.Conv2d(256, 64, 1),
            nn.ReLU(inplace=True),
            nn.Conv2d(64, 256, 1),
            nn.Sigmoid()
        )
        
        self.spatial_attention = nn.Sequential(
            nn.Conv2d(256, 64, 1),
            nn.ReLU(inplace=True),
            nn.Conv2d(64, 1, 1),
            nn.Sigmoid()
        )
        
        # 解码器
        self.decoder = nn.Sequential(
            nn.ConvTranspose2d(256, 128, 2, stride=2),
            nn.ReLU(inplace=True),
            nn.BatchNorm2d(128),
            
            nn.Conv2d(128, 128, 3, padding=1),
            nn.ReLU(inplace=True),
            nn.BatchNorm2d(128),
            
            nn.ConvTranspose2d(128, 64, 2, stride=2),
            nn.ReLU(inplace=True),
            nn.BatchNorm2d(64),
            
            nn.Conv2d(64, 64, 3, padding=1),
            nn.ReLU(inplace=True),
            nn.BatchNorm2d(64),
            
            nn.Conv2d(64, 1, 1),
        )
    
    def forward(self, x):
        # 编码
        enc = self.encoder(x)
        
        # 通道注意力
        ca = self.channel_attention(enc)
        enc = enc * ca
        
        # 空间注意力
        sa = self.spatial_attention(enc)
        enc = enc * sa
        
        # 解码
        out = self.decoder(enc)
        
        return torch.sigmoid(out)


class SyntheticCloudDataset(Dataset):
    """创建合成云分割数据集"""
    
    def __init__(self, num_samples=200, image_size=128, split='train'):
        self.num_samples = num_samples
        self.image_size = image_size
        self.split = split
        
        np.random.seed(42 if split == 'train' else 123)
        self.data = self._generate_data()
    
    def _generate_data(self):
        data = []
        
        for i in range(self.num_samples):
            # 创建4通道影像
            image = np.random.rand(self.image_size, self.image_size, 4).astype(np.float32)
            
            # 添加云状斑块
            for _ in range(np.random.randint(2, 4)):
                cx = np.random.randint(10, self.image_size-10)
                cy = np.random.randint(10, self.image_size-10)
                r = np.random.randint(8, 20)
                
                y, x = np.ogrid[:self.image_size, :self.image_size]
                mask = (x - cx)**2 + (y - cy)**2 <= r**2
                
                image[mask, :3] += np.random.uniform(0.3, 0.6)
                image[mask, 3] += np.random.uniform(0.4, 0.7)
            
            image = np.clip(image, 0, 1)
            
            # 创建掩码
            brightness = image[:, :, :3].mean(axis=2)
            cloud_mask = (brightness > 0.55).astype(np.float32)
            
            data.append((image, cloud_mask))
        
        return data
    
    def __len__(self):
        return self.num_samples
    
    def __getitem__(self, idx):
        image, mask = self.data[idx]
        
        image_tensor = torch.from_numpy(image).permute(2, 0, 1).float()
        mask_tensor = torch.from_numpy(mask).unsqueeze(0).float()
        
        if self.split == 'train':
            if np.random.random() > 0.5:
                image_tensor = torch.flip(image_tensor, dims=[2])
                mask_tensor = torch.flip(mask_tensor, dims=[2])
            if np.random.random() > 0.5:
                image_tensor = torch.flip(image_tensor, dims=[1])
                mask_tensor = torch.flip(mask_tensor, dims=[1])
        
        return image_tensor, mask_tensor


def train_lightweight():
    """训练轻量级模型"""
    
    print("=" * 80)
    print("SkySense++ 轻量级演示模型训练")
    print("=" * 80)
    print("\n使用轻量级CNN模型进行快速演示...")
    print("该模型包含语义增强模块，模拟SkySense++的核心功能。\n")
    
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f"使用设备: {device}")
    
    # 创建数据集
    print("\n创建合成数据集...")
    train_dataset = SyntheticCloudDataset(num_samples=200, image_size=128, split='train')
    val_dataset = SyntheticCloudDataset(num_samples=50, image_size=128, split='val')
    
    train_loader = DataLoader(train_dataset, batch_size=16, shuffle=True, num_workers=0)
    val_loader = DataLoader(val_dataset, batch_size=8, shuffle=False, num_workers=0)
    
    print(f"训练样本: {len(train_dataset)}, 验证样本: {len(val_dataset)}")
    
    # 创建模型
    print("\n创建轻量级模型 (模拟SkySense++)...")
    model = SimpleCNN(in_channels=4).to(device)
    
    n_params = sum(p.numel() for p in model.parameters())
    print(f"模型参数: {n_params:,}")
    
    # 训练设置
    criterion = nn.BCELoss()
    optimizer = optim.Adam(model.parameters(), lr=0.002)
    
    num_epochs = 8
    best_dice = 0.0
    
    print("\n" + "=" * 80)
    print("开始训练")
    print("=" * 80)
    
    train_losses = []
    val_dices = []
    
    for epoch in range(num_epochs):
        # 训练
        model.train()
        epoch_loss = 0.0
        
        pbar = tqdm(train_loader, desc=f"Epoch {epoch+1}/{num_epochs}")
        
        for images, masks in pbar:
            images = images.to(device)
            masks = masks.to(device)
            
            outputs = model(images)
            
            if outputs.shape != masks.shape:
                outputs = torch.nn.functional.interpolate(
                    outputs, size=masks.shape[2:], mode='bilinear', align_corners=True
                )
            
            loss = criterion(outputs, masks)
            
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()
            
            epoch_loss += loss.item()
            
            with torch.no_grad():
                pred = (outputs > 0.5).float()
                intersection = (pred * masks).sum()
                union = pred.sum() + masks.sum()
                dice = (2.0 * intersection) / (union + 1e-7)
            
            pbar.set_postfix({'loss': f'{loss.item():.4f}', 'dice': f'{dice.item():.4f}'})
        
        avg_loss = epoch_loss / len(train_loader)
        train_losses.append(avg_loss)
        
        # 验证
        print(f"\nEpoch {epoch+1}/{num_epochs} - Loss: {avg_loss:.4f}")
        
        model.eval()
        val_dice_total = 0.0
        
        with torch.no_grad():
            for val_images, val_masks in val_loader:
                val_images = val_images.to(device)
                val_masks = val_masks.to(device)
                
                val_outputs = model(val_images)
                
                if val_outputs.shape != val_masks.shape:
                    val_outputs = torch.nn.functional.interpolate(
                        val_outputs, size=val_masks.shape[2:], mode='bilinear', align_corners=True
                    )
                
                pred = (val_outputs > 0.5).float()
                intersection = (pred * val_masks).sum()
                union = pred.sum() + val_masks.sum()
                val_dice_total += (2.0 * intersection) / (union + 1e-7)
        
        avg_val_dice = val_dice_total / len(val_loader)
        val_dices.append(avg_val_dice.item())
        
        print(f"验证 Dice: {avg_val_dice:.4f}")
        
        # 保存最佳模型
        if avg_val_dice > best_dice:
            best_dice = avg_val_dice
            checkpoint_dir = project_root / "checkpoints"
            checkpoint_dir.mkdir(parents=True, exist_ok=True)
            torch.save(model.state_dict(), checkpoint_dir / "lightweight_best.pth")
            print(f"✓ 保存最佳模型 (Dice: {best_dice:.4f})")
    
    print("\n" + "=" * 80)
    print(f"训练完成！最佳 Dice: {best_dice:.4f}")
    print("=" * 80)
    
    # 可视化
    print("\n生成可视化结果...")
    output_dir = project_root / "demo_results"
    output_dir.mkdir(parents=True, exist_ok=True)
    
    # 加载最佳模型
    model.load_state_dict(torch.load(checkpoint_dir / "lightweight_best.pth", weights_only=True))
    model.eval()
    
    # 生成图像
    with torch.no_grad():
        for i, (images, masks) in enumerate(val_loader):
            if i >= 3:  # 只处理前3个批次
                break
            
            images = images.to(device)
            outputs = model(images)
            
            if outputs.shape != masks.shape:
                outputs = torch.nn.functional.interpolate(
                    outputs, size=masks.shape[2:], mode='bilinear', align_corners=True
                )
            
            for j in range(min(images.shape[0], 5)):
                fig, axes = plt.subplots(1, 4, figsize=(16, 4))
                
                # RGB
                img_rgb = images[j, 1:4].cpu().numpy()[::-1].transpose(1, 2, 0)
                img_rgb = np.clip(img_rgb, 0, 1)
                axes[0].imshow(img_rgb)
                axes[0].set_title("输入 (RGB)")
                axes[0].axis('off')
                
                # NIR
                axes[1].imshow(images[j, 3].cpu().numpy(), cmap='hot')
                axes[1].set_title("近红外 (NIR)")
                axes[1].axis('off')
                
                # GT
                axes[2].imshow(masks[j, 0].numpy(), cmap='gray')
                axes[2].set_title("真实掩码")
                axes[2].axis('off')
                
                # 预测
                pred = (outputs[j, 0].cpu().numpy() > 0.5).astype(np.float32)
                axes[3].imshow(pred, cmap='gray')
                axes[3].set_title("预测结果")
                axes[3].axis('off')
                
                plt.tight_layout()
                plt.savefig(output_dir / f"sample_{i*5+j+1:03d}.png", dpi=150, bbox_inches='tight')
                plt.close()
                
                print(f"  保存 sample_{i*5+j+1:03d}.png")
    
    # 绘制曲线
    fig, axes = plt.subplots(1, 2, figsize=(12, 4))
    
    axes[0].plot(range(1, len(train_losses)+1), train_losses, 'b-o', linewidth=2)
    axes[0].set_xlabel('Epoch')
    axes[0].set_ylabel('Loss')
    axes[0].set_title('训练损失')
    axes[0].grid(True, alpha=0.3)
    
    axes[1].plot(range(1, len(val_dices)+1), val_dices, 'r-o', linewidth=2)
    axes[1].set_xlabel('Epoch')
    axes[1].set_ylabel('Dice')
    axes[1].set_title('验证 Dice')
    axes[1].grid(True, alpha=0.3)
    axes[1].set_ylim([0, 1])
    
    plt.tight_layout()
    plt.savefig(output_dir / "training_curves.png", dpi=150)
    plt.close()
    
    print(f"\n✓ 可视化结果已保存至: {output_dir}")
    print("\n" + "=" * 80)
    print("演示完成！")
    print("=" * 80)
    print(f"\n生成的文件:")
    print(f"  - 模型: checkpoints/lightweight_best.pth")
    print(f"  - 图像: demo_results/")
    print(f"\n查看图像: ls demo_results/")


if __name__ == "__main__":
    train_lightweight()
