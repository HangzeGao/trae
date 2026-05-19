#!/usr/bin/env python3
"""
使用已有数据和完整SkySense++模型进行训练和测试
"""

import sys
import numpy as np
import random
from pathlib import Path
from tqdm import tqdm
import matplotlib.pyplot as plt

import torch
import torch.nn as nn
import torch.optim as optim
from torch.utils.data import Dataset, DataLoader
from PIL import Image

project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

class ExistingCloudDataset(Dataset):
    """使用已有数据的数据集"""
    
    def __init__(self, data_dir, split='train', augment=True):
        self.data_dir = Path(data_dir)
        self.split = split
        self.augment = augment
        
        # 获取所有样本
        self.samples = sorted([d for d in self.data_dir.iterdir() if d.is_dir()])
        print(f"找到 {len(self.samples)} 个样本")
        
        # 划分数据集
        random.seed(42)
        indices = list(range(len(self.samples)))
        random.shuffle(indices)
        
        train_size = int(0.7 * len(self.samples))
        val_size = int(0.15 * len(self.samples))
        
        if split == 'train':
            self.samples = [self.samples[i] for i in indices[:train_size]]
        elif split == 'val':
            self.samples = [self.samples[i] for i in indices[train_size:train_size+val_size]]
        else:  # test
            self.samples = [self.samples[i] for i in indices[train_size+val_size:]]
        
        print(f"{split}: {len(self.samples)} 样本")
    
    def __len__(self):
        return len(self.samples)
    
    def __getitem__(self, idx):
        sample_dir = self.samples[idx]
        
        # 加载RGB图像
        rgb_img = np.array(Image.open(sample_dir / 'rgb.png'))
        if len(rgb_img.shape) == 2:
            rgb_img = np.stack([rgb_img] * 3, axis=-1)
        
        # 加载NIR图像
        nir_img = np.array(Image.open(sample_dir / 'nir.png'))
        if len(nir_img.shape) == 3:
            nir_img = nir_img[:, :, 0]
        
        # 堆叠为4通道 (RGB + NIR)
        image = np.concatenate([rgb_img, nir_img[:, :, np.newaxis]], axis=-1).astype(np.float32)
        
        # 加载掩码
        mask = np.array(Image.open(sample_dir / 'mask.png'))
        if len(mask.shape) == 3:
            mask = mask[:, :, 0]
        mask = (mask > 127).astype(np.float32)
        
        # 归一化
        if image.max() > 1:
            image = image / 255.0
        
        # 转换为tensor
        image_tensor = torch.from_numpy(image).permute(2, 0, 1).float()
        mask_tensor = torch.from_numpy(mask).unsqueeze(0).float()
        
        # 调整大小
        target_size = 256
        if image_tensor.shape[1] != target_size:
            image_tensor = torch.nn.functional.interpolate(
                image_tensor.unsqueeze(0), 
                size=(target_size, target_size), 
                mode='bilinear', 
                align_corners=True
            ).squeeze(0)
            mask_tensor = torch.nn.functional.interpolate(
                mask_tensor.unsqueeze(0), 
                size=(target_size, target_size), 
                mode='nearest'
            ).squeeze(0)
        
        # 数据增强
        if self.augment and self.split == 'train':
            if random.random() > 0.5:
                image_tensor = torch.flip(image_tensor, dims=[2])
                mask_tensor = torch.flip(mask_tensor, dims=[2])
            if random.random() > 0.5:
                image_tensor = torch.flip(image_tensor, dims=[1])
                mask_tensor = torch.flip(mask_tensor, dims=[1])
        
        return image_tensor, mask_tensor

def train_with_existing_data():
    """使用已有数据训练完整SkySense++模型"""
    
    print("=" * 80)
    print("完整SkySense++训练流程")
    print("=" * 80)
    print("\n使用已有真实云分割数据")
    print("数据来源: data/clouds_for_training/\n")
    
    # 导入模型
    print("加载SkySense++模型...")
    from models.skysense_pp import SkySensePPModel
    
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f"使用设备: {device}")
    
    # 创建模型
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
    
    # 创建数据集
    print("\n加载数据集...")
    data_dir = project_root / "data" / "clouds_for_training"
    
    train_dataset = ExistingCloudDataset(data_dir, split='train', augment=True)
    val_dataset = ExistingCloudDataset(data_dir, split='val', augment=False)
    test_dataset = ExistingCloudDataset(data_dir, split='test', augment=False)
    
    train_loader = DataLoader(train_dataset, batch_size=4, shuffle=True, num_workers=0)
    val_loader = DataLoader(val_dataset, batch_size=2, shuffle=False, num_workers=0)
    test_loader = DataLoader(test_dataset, batch_size=2, shuffle=False, num_workers=0)
    
    print(f"\n训练批次: {len(train_loader)}, 验证批次: {len(val_loader)}, 测试批次: {len(test_loader)}")
    
    # 训练设置
    criterion = nn.BCELoss()
    optimizer = optim.Adam(model.parameters(), lr=0.001)
    
    num_epochs = 10
    best_dice = 0.0
    
    checkpoint_dir = project_root / "checkpoints"
    checkpoint_dir.mkdir(parents=True, exist_ok=True)
    
    print("\n" + "=" * 80)
    print("开始训练")
    print("=" * 80)
    
    for epoch in range(num_epochs):
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
        
        # 验证
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
        
        print(f"\nEpoch {epoch+1}/{num_epochs} - Loss: {avg_loss:.4f}, Val Dice: {avg_val_dice:.4f}")
        
        if avg_val_dice > best_dice:
            best_dice = avg_val_dice
            torch.save(model.state_dict(), checkpoint_dir / "skysensepp_existing_best.pth")
            print(f"  ✓ 保存最佳模型 (Dice: {best_dice:.4f})")
    
    # 测试
    print("\n" + "=" * 80)
    print("测试模型 (未参与训练的数据)")
    print("=" * 80)
    
    model.load_state_dict(torch.load(checkpoint_dir / "skysensepp_existing_best.pth", weights_only=True))
    model.eval()
    
    test_dice_total = 0.0
    test_iou_total = 0.0
    test_count = 0
    
    output_dir = project_root / "final_test_results"
    output_dir.mkdir(parents=True, exist_ok=True)
    
    with torch.no_grad():
        pbar = tqdm(test_loader, desc="测试中")
        
        for batch_idx, (images, masks) in enumerate(pbar):
            images = images.to(device)
            masks = masks.to(device)
            
            outputs = model(images)
            
            if outputs.shape != masks.shape:
                outputs = torch.nn.functional.interpolate(
                    outputs, size=masks.shape[2:], mode='bilinear', align_corners=True
                )
            
            pred = (outputs > 0.5).float()
            intersection = (pred * masks).sum()
            union = pred.sum() + masks.sum()
            dice = (2.0 * intersection) / (union + 1e-7)
            
            intersection_iou = (pred * masks).sum()
            union_iou = pred.sum() + masks.sum() - intersection_iou
            iou = intersection_iou / (union_iou + 1e-7)
            
            test_dice_total += dice.item()
            test_iou_total += iou.item()
            test_count += 1
            
            # 保存可视化
            images_np = images.cpu().numpy()
            masks_np = masks.cpu().numpy()
            outputs_np = outputs.cpu().numpy()
            
            for i in range(images_np.shape[0]):
                fig, axes = plt.subplots(1, 4, figsize=(16, 4))
                
                # RGB
                img_rgb = images_np[i, :3][::-1].transpose(1, 2, 0)
                img_rgb = np.clip(img_rgb, 0, 1)
                axes[0].imshow(img_rgb)
                axes[0].set_title("Input (RGB)", fontsize=11)
                axes[0].axis('off')
                
                # NIR
                axes[1].imshow(images_np[i, 3], cmap='hot')
                axes[1].set_title("NIR", fontsize=11)
                axes[1].axis('off')
                
                # GT
                axes[2].imshow(masks_np[i, 0], cmap='gray')
                axes[2].set_title("Ground Truth", fontsize=11)
                axes[2].axis('off')
                
                # Prediction
                pred_binary = (outputs_np[i, 0] > 0.5).astype(np.float32)
                axes[3].imshow(pred_binary, cmap='gray')
                axes[3].set_title(f"Prediction (Dice: {dice.item():.3f})", fontsize=11)
                axes[3].axis('off')
                
                plt.suptitle(f"Test Sample {batch_idx*2+i+1}", fontsize=12, fontweight='bold')
                plt.tight_layout()
                plt.savefig(output_dir / f"test_{batch_idx*2+i+1:03d}.png", dpi=150, bbox_inches='tight')
                plt.close()
    
    avg_test_dice = test_dice_total / test_count
    avg_test_iou = test_iou_total / test_count
    
    # 保存测试总结
    fig, axes = plt.subplots(1, 2, figsize=(12, 5))
    
    axes[0].bar(['Dice', 'IoU'], [avg_test_dice, avg_test_iou], color=['#2E86AB', '#A23B72'])
    axes[0].set_ylabel('Score')
    axes[0].set_title('Test Performance Metrics')
    axes[0].set_ylim([0, 1])
    for i, v in enumerate([avg_test_dice, avg_test_iou]):
        axes[0].text(i, v + 0.02, f'{v:.4f}', ha='center', fontsize=12, fontweight='bold')
    
    axes[1].text(0.5, 0.5, 
                  f'Test Results Summary\n\n'
                  f'Dice Coefficient: {avg_test_dice:.4f}\n'
                  f'IoU Coefficient: {avg_test_iou:.4f}\n\n'
                  f'Test Samples: {len(test_dataset)}\n'
                  f'Model: SkySense++\n'
                  f'Architecture: Adaptive Fusion + Semantic Enhancement',
                  ha='center', va='center', fontsize=12, transform=axes[1].transAxes,
                  bbox=dict(boxstyle='round', facecolor='wheat', alpha=0.5))
    axes[1].axis('off')
    
    plt.tight_layout()
    plt.savefig(output_dir / "test_summary.png", dpi=150, bbox_inches='tight')
    plt.close()
    
    print(f"\n测试完成!")
    print(f"测试 Dice: {avg_test_dice:.4f}")
    print(f"测试 IoU: {avg_test_iou:.4f}")
    print(f"\n可视化结果已保存至: {output_dir}")
    
    # 保存详细报告
    report = f"""# SkySense++ 训练和测试报告

## 配置
- 模型: SkySense++
- 编码器: ResNet-34
- 输入通道: 4 (RGB + NIR)
- 输出: 1 (云分割)
- 融合方式: Adaptive Fusion
- 语义增强: 启用
- 模型参数: {n_params:,}

## 数据集
- 总样本: {len(train_dataset) + len(val_dataset) + len(test_dataset)}
- 训练集: {len(train_dataset)} 样本
- 验证集: {len(val_dataset)} 样本
- 测试集: {len(test_dataset)} 样本

## 训练结果
- 最佳验证Dice: {best_dice:.4f}
- 训练轮数: {num_epochs}

## 测试结果
- 测试Dice: {avg_test_dice:.4f}
- 测试IoU: {avg_test_iou:.4f}

## 生成文件
- 模型: checkpoints/skysensepp_existing_best.pth
- 可视化: final_test_results/
- 报告: test_report.md

训练完成时间: {Path.cwd()}
"""
    
    with open(output_dir / "test_report.md", 'w') as f:
        f.write(report)
    
    print(f"详细报告已保存至: {output_dir}/test_report.md")
    
    return model, avg_test_dice, avg_test_iou

if __name__ == "__main__":
    model, dice, iou = train_with_existing_data()
    print(f"\n最终测试结果:")
    print(f"  Dice: {dice:.4f}")
    print(f"  IoU: {iou:.4f}")
