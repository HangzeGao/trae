#!/usr/bin/env python3
"""
使用合成数据进行 SkySense++ 快速演示
创建模拟的云分割数据来验证训练流程
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

# 添加项目路径
project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

class SyntheticCloudDataset(Dataset):
    """创建合成云分割数据集用于演示"""
    
    def __init__(self, num_samples=200, image_size=256, split='train'):
        self.num_samples = num_samples
        self.image_size = image_size
        self.split = split
        
        # 设置随机种子以保证可重复性
        np.random.seed(42 if split == 'train' else 123)
        
        # 生成合成数据
        self.data = self._generate_data()
    
    def _generate_data(self):
        """生成合成遥感影像和云掩码"""
        data = []
        
        for i in range(self.num_samples):
            # 创建4通道影像 (模拟 Blue, Green, Red, NIR)
            image = np.random.rand(self.image_size, self.image_size, 4).astype(np.float32)
            
            # 添加一些结构（模拟真实遥感数据）
            # 添加云状斑块
            for _ in range(np.random.randint(2, 5)):
                center_x = np.random.randint(0, self.image_size)
                center_y = np.random.randint(0, self.image_size)
                radius = np.random.randint(20, 50)
                
                y, x = np.ogrid[:self.image_size, :self.image_size]
                mask_circle = (x - center_x)**2 + (y - center_y)**2 <= radius**2
                
                # 云在RGB上较亮，在NIR上更亮
                image[mask_circle, :3] += np.random.uniform(0.3, 0.7)
                image[mask_circle, 3] += np.random.uniform(0.4, 0.8)
            
            # 限制范围
            image = np.clip(image, 0, 1)
            
            # 创建云掩码 (基于阈值)
            brightness = image[:, :, :3].mean(axis=2)
            cloud_mask = (brightness > 0.6).astype(np.float32)
            
            # 添加一些噪声
            noise = np.random.normal(0, 0.05, cloud_mask.shape).astype(np.float32)
            cloud_mask = np.clip(cloud_mask + noise, 0, 1)
            cloud_mask = (cloud_mask > 0.5).astype(np.float32)
            
            data.append((image, cloud_mask))
        
        return data
    
    def __len__(self):
        return self.num_samples
    
    def __getitem__(self, idx):
        image, mask = self.data[idx]
        
        # 转换为tensor
        image_tensor = torch.from_numpy(image).permute(2, 0, 1).float()
        mask_tensor = torch.from_numpy(mask).unsqueeze(0).float()
        
        # 数据增强 (仅训练集)
        if self.split == 'train':
            if np.random.random() > 0.5:
                image_tensor = torch.flip(image_tensor, dims=[2])
                mask_tensor = torch.flip(mask_tensor, dims=[2])
            
            if np.random.random() > 0.5:
                image_tensor = torch.flip(image_tensor, dims=[1])
                mask_tensor = torch.flip(mask_tensor, dims=[1])
        
        return image_tensor, mask_tensor


def train_with_synthetic_data():
    """使用合成数据训练 SkySense++"""
    
    print("=" * 80)
    print("SkySense++ 合成数据演示训练")
    print("=" * 80)
    print("\n这个演示使用合成的云分割数据来展示完整的训练流程。")
    print("在实际应用中，请使用真实的 38Cloud 数据集。\n")
    
    # 导入 SkySense++ 模型
    print("加载 SkySense++ 模型...")
    from models.skysense_pp import SkySensePPModel
    
    # 创建设备
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f"使用设备: {device}")
    
    # 创建数据加载器
    print("\n创建合成数据集...")
    train_dataset = SyntheticCloudDataset(num_samples=200, image_size=256, split='train')
    val_dataset = SyntheticCloudDataset(num_samples=50, image_size=256, split='val')
    
    train_loader = DataLoader(train_dataset, batch_size=8, shuffle=True, num_workers=0)
    val_loader = DataLoader(val_dataset, batch_size=4, shuffle=False, num_workers=0)
    
    print(f"训练样本: {len(train_dataset)}, 验证样本: {len(val_dataset)}")
    print(f"训练批次: {len(train_loader)}, 验证批次: {len(val_loader)}")
    
    # 创建模型
    print("\n创建 SkySense++ 模型...")
    model = SkySensePPModel(
        encoder_name='resnet34',
        in_channels=4,
        num_classes=1,
        fusion_type='adaptive',
        use_semantic_enhancement=True,
        dropout=0.1,
        activation='sigmoid'
    ).to(device)
    
    n_params = sum(p.numel() for p in model.parameters())
    print(f"模型参数: {n_params:,}")
    
    # 损失函数和优化器
    criterion = nn.BCELoss()
    optimizer = optim.Adam(model.parameters(), lr=0.001)
    
    # 训练设置
    num_epochs = 10
    best_dice = 0.0
    checkpoint_dir = project_root / "checkpoints"
    checkpoint_dir.mkdir(parents=True, exist_ok=True)
    
    train_losses = []
    val_dices = []
    
    print("\n" + "=" * 80)
    print("开始训练")
    print("=" * 80)
    
    for epoch in range(num_epochs):
        # 训练阶段
        model.train()
        epoch_loss = 0.0
        
        pbar = tqdm(train_loader, desc=f"Epoch {epoch+1}/{num_epochs}")
        
        for images, masks in pbar:
            images = images.to(device)
            masks = masks.to(device)
            
            # 前向传播
            outputs = model(images)
            
            # 确保形状一致
            if outputs.shape != masks.shape:
                outputs = torch.nn.functional.interpolate(
                    outputs, size=masks.shape[2:], mode='bilinear', align_corners=True
                )
            
            loss = criterion(outputs, masks)
            
            # 反向传播
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()
            
            epoch_loss += loss.item()
            
            # 计算训练Dice
            with torch.no_grad():
                pred_binary = (outputs > 0.5).float()
                intersection = (pred_binary * masks).sum()
                union = pred_binary.sum() + masks.sum()
                dice = (2.0 * intersection) / (union + 1e-7)
            
            pbar.set_postfix({
                'loss': f'{loss.item():.4f}',
                'dice': f'{dice.item():.4f}'
            })
        
        avg_train_loss = epoch_loss / len(train_loader)
        train_losses.append(avg_train_loss)
        
        # 验证阶段
        print(f"\nEpoch {epoch+1}/{num_epochs} - 训练 Loss: {avg_train_loss:.4f}")
        print("验证中...")
        
        model.eval()
        val_dice_total = 0.0
        val_iou_total = 0.0
        
        with torch.no_grad():
            for val_images, val_masks in val_loader:
                val_images = val_images.to(device)
                val_masks = val_masks.to(device)
                
                val_outputs = model(val_images)
                
                if val_outputs.shape != val_masks.shape:
                    val_outputs = torch.nn.functional.interpolate(
                        val_outputs, size=val_masks.shape[2:], mode='bilinear', align_corners=True
                    )
                
                # 计算指标
                pred_binary = (val_outputs > 0.5).float()
                intersection = (pred_binary * val_masks).sum()
                union = pred_binary.sum() + val_masks.sum()
                val_dice_total += (2.0 * intersection) / (union + 1e-7)
                
                # IoU
                intersection_iou = (pred_binary * val_masks).sum()
                union_iou = pred_binary.sum() + val_masks.sum() - intersection_iou
                val_iou_total += intersection_iou / (union_iou + 1e-7)
        
        avg_val_dice = val_dice_total / len(val_loader)
        avg_val_iou = val_iou_total / len(val_loader)
        val_dices.append(avg_val_dice.item())
        
        print(f"验证 Dice: {avg_val_dice:.4f}, IoU: {avg_val_iou:.4f}")
        
        # 保存最佳模型
        if avg_val_dice > best_dice:
            best_dice = avg_val_dice
            checkpoint_path = checkpoint_dir / "skysensepp_synthetic_best.pth"
            torch.save({
                'epoch': epoch + 1,
                'model_state_dict': model.state_dict(),
                'optimizer_state_dict': optimizer.state_dict(),
                'dice': best_dice,
                'iou': avg_val_iou,
            }, checkpoint_path)
            print(f"✓ 保存最佳模型 (Dice: {best_dice:.4f})")
    
    print("\n" + "=" * 80)
    print("训练完成！")
    print(f"最佳验证 Dice: {best_dice:.4f}")
    print("=" * 80)
    
    # 生成可视化
    print("\n" + "=" * 80)
    print("生成验证结果可视化")
    print("=" * 80)
    
    output_dir = project_root / "synthetic_results"
    output_dir.mkdir(parents=True, exist_ok=True)
    
    # 加载最佳模型
    best_checkpoint = checkpoint_dir / "skysensepp_synthetic_best.pth"
    checkpoint = torch.load(best_checkpoint, map_location=device)
    model.load_state_dict(checkpoint['model_state_dict'])
    model.eval()
    
    # 生成可视化
    visualize_results(model, val_loader, device, output_dir)
    
    # 绘制训练曲线
    plot_curves(train_losses, val_dices, output_dir)
    
    print(f"\n✓ 结果已保存至: {output_dir}")
    print("\n演示完成！")
    print("\n下一步:")
    print("1. 查看生成的图像: ls synthetic_results/")
    print("2. 下载真实数据集后，运行: python run_skysensepp_38cloud.py")
    
    return model, output_dir

def visualize_results(model, dataloader, device, output_dir, num_samples=15):
    """可视化预测结果"""
    model.eval()
    samples_saved = 0
    
    with torch.no_grad():
        for images, masks in dataloader:
            images = images.to(device)
            masks = masks.to(device)
            
            outputs = model(images)
            
            # 确保形状一致
            if outputs.shape != masks.shape:
                outputs = torch.nn.functional.interpolate(
                    outputs, size=masks.shape[2:], mode='bilinear', align_corners=True
                )
            
            # 转换为numpy
            images_np = images.cpu().numpy()
            masks_np = masks.cpu().numpy()
            outputs_np = outputs.cpu().numpy()
            
            for i in range(images_np.shape[0]):
                if samples_saved >= num_samples:
                    break
                
                fig, axes = plt.subplots(1, 4, figsize=(16, 4))
                
                # 输入RGB
                img_rgb = images_np[i, 1:4][::-1].transpose(1, 2, 0)
                img_rgb = np.clip(img_rgb, 0, 1)
                axes[0].imshow(img_rgb)
                axes[0].set_title("输入 (RGB)", fontsize=11)
                axes[0].axis('off')
                
                # NIR
                img_nir = images_np[i, 3]
                axes[1].imshow(img_nir, cmap='hot')
                axes[1].set_title("近红外 (NIR)", fontsize=11)
                axes[1].axis('off')
                
                # 真实掩码
                mask = masks_np[i, 0]
                axes[2].imshow(mask, cmap='gray')
                axes[2].set_title("真实掩码 (GT)", fontsize=11)
                axes[2].axis('off')
                
                # 预测
                pred = outputs_np[i, 0]
                pred_binary = (pred > 0.5).astype(np.float32)
                axes[3].imshow(pred_binary, cmap='gray')
                axes[3].set_title("SkySense++ 预测", fontsize=11)
                axes[3].axis('off')
                
                plt.suptitle(f"样本 {samples_saved+1}", fontsize=12, fontweight='bold')
                plt.tight_layout()
                
                save_path = output_dir / f"result_{samples_saved+1:03d}.png"
                plt.savefig(save_path, dpi=150, bbox_inches='tight')
                plt.close()
                
                print(f"  保存 {save_path}")
                samples_saved += 1
            
            if samples_saved >= num_samples:
                break
    
    print(f"\n✓ 保存了 {samples_saved} 个可视化结果")

def plot_curves(train_losses, val_dices, output_dir):
    """绘制训练曲线"""
    fig, axes = plt.subplots(1, 2, figsize=(14, 5))
    
    # Loss曲线
    axes[0].plot(range(1, len(train_losses)+1), train_losses, 'b-o', linewidth=2, markersize=8)
    axes[0].set_xlabel('Epoch', fontsize=11)
    axes[0].set_ylabel('Loss', fontsize=11)
    axes[0].set_title('训练损失曲线', fontsize=12, fontweight='bold')
    axes[0].grid(True, alpha=0.3)
    axes[0].set_xticks(range(1, len(train_losses)+1))
    
    # Dice曲线
    axes[1].plot(range(1, len(val_dices)+1), val_dices, 'r-o', linewidth=2, markersize=8, label='验证 Dice')
    axes[1].set_xlabel('Epoch', fontsize=11)
    axes[1].set_ylabel('Dice Coefficient', fontsize=11)
    axes[1].set_title('验证 Dice 系数', fontsize=12, fontweight='bold')
    axes[1].legend(loc='lower right')
    axes[1].grid(True, alpha=0.3)
    axes[1].set_xticks(range(1, len(val_dices)+1))
    axes[1].set_ylim([0, 1])
    
    plt.tight_layout()
    save_path = output_dir / "training_curves.png"
    plt.savefig(save_path, dpi=150, bbox_inches='tight')
    plt.close()
    
    print(f"✓ 训练曲线已保存至: {save_path}")

if __name__ == "__main__":
    train_with_synthetic_data()
