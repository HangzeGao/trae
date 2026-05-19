#!/usr/bin/env python3
"""
完整训练流程: 下载数据 -> 训练 -> 测试 -> 可视化
使用完整的SkySense++模型
"""

import sys
import os
from pathlib import Path
import numpy as np
import random
from tqdm import tqdm
import matplotlib.pyplot as plt

import torch
import torch.nn as nn
import torch.optim as optim
from torch.utils.data import Dataset, DataLoader, Subset
from PIL import Image

# 添加项目路径
project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

def download_38cloud():
    """下载38Cloud数据集"""
    print("=" * 80)
    print("步骤1: 下载 38Cloud 数据集")
    print("=" * 80)
    
    try:
        import kagglehub
        path = kagglehub.dataset_download("sorour/38cloud-cloud-segmentation-in-satellite-images")
        print(f"✓ 数据集下载至: {path}")
        return Path(path)
    except Exception as e:
        print(f"✗ 下载失败: {e}")
        sys.exit(1)

def find_dataset_structure(dataset_path):
    """查找数据集目录结构"""
    print("\n分析数据集结构...")
    
    for root, dirs, files in os.walk(dataset_path):
        # 查找包含图像的目录
        if 'blue' in [d.lower() for d in dirs] or any('blue' in f.lower() for f in files):
            print(f"找到数据目录: {root}")
            return Path(root)
    
    return None

class RealCloudDataset(Dataset):
    """真实的38Cloud数据集加载器"""
    
    def __init__(self, dataset_path, patch_list, image_size=256, split='train', augment=True):
        self.dataset_path = Path(dataset_path)
        self.patch_list = patch_list
        self.image_size = image_size
        self.split = split
        self.augment = augment
        self.bands = ['blue', 'green', 'red', 'nir']
        
        # 查找训练目录
        self.train_dir = self._find_train_dir()
        print(f"训练目录: {self.train_dir}")
    
    def _find_train_dir(self):
        candidates = [
            self.dataset_path / "38-Cloud_training",
            self.dataset_path,
        ]
        for cand in candidates:
            if cand.exists():
                return cand
        return self.dataset_path
    
    def _find_file(self, band, patch_id):
        """查找波段文件"""
        candidates = []
        
        # 多种可能的路径模式
        patterns = [
            f"train_{band}/{band}_patch_{patch_id}.TIF",
            f"train_{band}/{band}_{patch_id}.TIF",
            f"{band}/patch_{patch_id}.TIF",
            f"{band}/{band}_patch_{patch_id}.TIF",
        ]
        
        for pattern in patterns:
            candidates.append(self.train_dir / pattern)
        
        for cand in candidates:
            if cand.exists():
                return cand
        
        # 递归搜索
        for ext in ['*.TIF', '*.tif', '*.png']:
            for f in self.train_dir.rglob(f"*{band}*patch*{patch_id}*{ext}"):
                return f
        
        return None
    
    def _find_mask(self, patch_id):
        """查找掩码文件"""
        candidates = [
            self.train_dir / f"train_gt/gt_patch_{patch_id}.TIF",
            self.train_dir / f"train_gt/gt_{patch_id}.TIF",
            self.train_dir / f"gt/patch_{patch_id}.TIF",
            self.train_dir / f"gt/gt_patch_{patch_id}.TIF",
        ]
        
        for cand in candidates:
            if cand.exists():
                return cand
        
        for ext in ['*.TIF', '*.tif', '*.png']:
            for f in self.train_dir.rglob(f"*gt*patch*{patch_id}*{ext}"):
                return f
        
        return None
    
    def __len__(self):
        return len(self.patch_list)
    
    def __getitem__(self, idx):
        patch_id = self.patch_list[idx]
        
        # 加载4个波段
        bands_data = []
        for band in self.bands:
            band_file = self._find_file(band, patch_id)
            if band_file is not None:
                img = np.array(Image.open(band_file))
                if len(img.shape) == 2:
                    img = np.stack([img] * 3, axis=-1)
            else:
                img = np.zeros((self.image_size, self.image_size, 3), dtype=np.uint8)
            bands_data.append(img[:, :, :3])  # 取前3个通道
        
        # 堆叠为4通道
        image = np.concatenate(bands_data, axis=-1).astype(np.float32)
        
        # 加载掩码
        mask_file = self._find_mask(patch_id)
        if mask_file is not None:
            mask = np.array(Image.open(mask_file))
            if len(mask.shape) == 3:
                mask = mask[:, :, 0]
            mask = (mask > 127).astype(np.float32)
        else:
            mask = np.zeros((self.image_size, self.image_size), dtype=np.float32)
        
        # 归一化
        if image.max() > 1:
            image = image / 255.0
        
        # 转换为tensor
        image_tensor = torch.from_numpy(image).permute(2, 0, 1).float()
        mask_tensor = torch.from_numpy(mask).unsqueeze(0).float()
        
        # 调整大小
        if image_tensor.shape[1] != self.image_size:
            image_tensor = torch.nn.functional.interpolate(
                image_tensor.unsqueeze(0), 
                size=(self.image_size, self.image_size), 
                mode='bilinear', 
                align_corners=True
            ).squeeze(0)
            mask_tensor = torch.nn.functional.interpolate(
                mask_tensor.unsqueeze(0), 
                size=(self.image_size, self.image_size), 
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

def get_patches_from_dataset(dataset_path):
    """从数据集获取所有patch ID"""
    patches = set()
    
    train_dir = dataset_path / "38-Cloud_training" if (dataset_path / "38-Cloud_training").exists() else dataset_path
    
    # 搜索蓝色波段文件
    for f in train_dir.rglob("blue_patch_*.TIF"):
        patch_id = f.stem.replace("blue_patch_", "")
        patches.add(patch_id)
    
    for f in train_dir.rglob("blue_*.TIF"):
        name = f.stem
        if name.startswith("blue_"):
            patch_id = name.replace("blue_", "")
            patches.add(patch_id)
    
    return list(patches)

def train_and_test():
    """完整训练和测试流程"""
    
    # 步骤1: 下载数据
    dataset_path = download_38cloud()
    
    # 步骤2: 获取所有patch
    print("\n" + "=" * 80)
    print("步骤2: 扫描数据集")
    print("=" * 80)
    
    all_patches = get_patches_from_dataset(dataset_path)
    print(f"找到 {len(all_patches)} 个patches")
    
    if len(all_patches) == 0:
        print("✗ 未找到任何patches！")
        sys.exit(1)
    
    # 步骤3: 划分训练和测试集
    print("\n" + "=" * 80)
    print("步骤3: 划分数据集 (70%训练, 30%测试)")
    print("=" * 80)
    
    random.seed(42)
    random.shuffle(all_patches)
    
    train_size = int(0.7 * len(all_patches))
    train_patches = all_patches[:train_size]
    test_patches = all_patches[train_size:]
    
    # 从训练集中再划分验证集
    val_size = int(0.15 * len(train_patches))
    val_patches = train_patches[:val_size]
    train_patches_final = train_patches[val_size:]
    
    print(f"训练集: {len(train_patches_final)} patches")
    print(f"验证集: {len(val_patches)} patches")
    print(f"测试集: {len(test_patches)} patches")
    
    # 限制训练数量以适应磁盘空间
    max_train = 500
    max_val = 50
    max_test = 100
    
    if len(train_patches_final) > max_train:
        train_patches_final = train_patches_final[:max_train]
        print(f"限制训练集到 {max_train} patches")
    
    if len(val_patches) > max_val:
        val_patches = val_patches[:max_val]
        print(f"限制验证集到 {max_val} patches")
    
    if len(test_patches) > max_test:
        test_patches = test_patches[:max_test]
        print(f"限制测试集到 {max_test} patches")
    
    # 步骤4: 加载SkySense++模型
    print("\n" + "=" * 80)
    print("步骤4: 加载完整 SkySense++ 模型")
    print("=" * 80)
    
    from models.skysense_pp import SkySensePPModel
    
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f"使用设备: {device}")
    
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
    
    # 步骤5: 创建数据加载器
    print("\n" + "=" * 80)
    print("步骤5: 创建数据加载器")
    print("=" * 80)
    
    image_size = 256
    batch_size = 4
    
    train_dataset = RealCloudDataset(dataset_path, train_patches_final, image_size=image_size, split='train')
    val_dataset = RealCloudDataset(dataset_path, val_patches, image_size=image_size, split='val', augment=False)
    test_dataset = RealCloudDataset(dataset_path, test_patches, image_size=image_size, split='test', augment=False)
    
    train_loader = DataLoader(train_dataset, batch_size=batch_size, shuffle=True, num_workers=0)
    val_loader = DataLoader(val_dataset, batch_size=batch_size, shuffle=False, num_workers=0)
    test_loader = DataLoader(test_dataset, batch_size=batch_size, shuffle=False, num_workers=0)
    
    print(f"训练批次: {len(train_loader)}, 验证批次: {len(val_loader)}, 测试批次: {len(test_loader)}")
    
    # 步骤6: 训练
    print("\n" + "=" * 80)
    print("步骤6: 训练模型")
    print("=" * 80)
    
    criterion = nn.BCELoss()
    optimizer = optim.Adam(model.parameters(), lr=0.001)
    
    num_epochs = 5
    best_dice = 0.0
    
    checkpoint_dir = project_root / "checkpoints"
    checkpoint_dir.mkdir(parents=True, exist_ok=True)
    
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
        
        print(f"Epoch {epoch+1}/{num_epochs} - Loss: {avg_loss:.4f}, Val Dice: {avg_val_dice:.4f}")
        
        if avg_val_dice > best_dice:
            best_dice = avg_val_dice
            torch.save(model.state_dict(), checkpoint_dir / "skysensepp_real_best.pth")
            print(f"  ✓ 保存最佳模型 (Dice: {best_dice:.4f})")
    
    # 步骤7: 测试
    print("\n" + "=" * 80)
    print("步骤7: 测试模型 (使用未参与训练的数据)")
    print("=" * 80)
    
    model.load_state_dict(torch.load(checkpoint_dir / "skysensepp_real_best.pth", weights_only=True))
    model.eval()
    
    test_dice_total = 0.0
    test_iou_total = 0.0
    test_count = 0
    
    output_dir = project_root / "test_results"
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
            
            # 计算指标
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
            
            # 保存可视化结果
            if batch_idx < 10:  # 只保存前10个批次
                images_np = images.cpu().numpy()
                masks_np = masks.cpu().numpy()
                outputs_np = outputs.cpu().numpy()
                
                for i in range(min(images_np.shape[0], 3)):
                    fig, axes = plt.subplots(1, 4, figsize=(16, 4))
                    
                    # RGB
                    img_rgb = images_np[i, 1:4][::-1].transpose(1, 2, 0)
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
                    
                    plt.tight_layout()
                    plt.savefig(output_dir / f"test_{batch_idx*3+i+1:03d}.png", dpi=150, bbox_inches='tight')
                    plt.close()
    
    avg_test_dice = test_dice_total / test_count
    avg_test_iou = test_iou_total / test_count
    
    print(f"\n测试结果:")
    print(f"  Dice系数: {avg_test_dice:.4f}")
    print(f"  IoU系数: {avg_test_iou:.4f}")
    
    # 绘制测试结果分布
    fig, axes = plt.subplots(1, 2, figsize=(12, 5))
    
    axes[0].bar(['Dice', 'IoU'], [avg_test_dice, avg_test_iou], color=['#2E86AB', '#A23B72'])
    axes[0].set_ylabel('Score')
    axes[0].set_title('Test Metrics')
    axes[0].set_ylim([0, 1])
    for i, v in enumerate([avg_test_dice, avg_test_iou]):
        axes[0].text(i, v + 0.02, f'{v:.4f}', ha='center', fontsize=12, fontweight='bold')
    
    axes[1].text(0.5, 0.5, f'Test Results\n\nDice: {avg_test_dice:.4f}\nIoU: {avg_test_iou:.4f}\n\nTest Samples: {len(test_patches)}\nModel: SkySense++',
                 ha='center', va='center', fontsize=14, transform=axes[1].transAxes,
                 bbox=dict(boxstyle='round', facecolor='wheat', alpha=0.5))
    axes[1].axis('off')
    
    plt.tight_layout()
    plt.savefig(output_dir / "test_summary.png", dpi=150)
    plt.close()
    
    print(f"\n✓ 测试可视化结果已保存至: {output_dir}")
    print("\n" + "=" * 80)
    print("训练和测试完成！")
    print("=" * 80)
    print(f"最佳验证Dice: {best_dice:.4f}")
    print(f"测试Dice: {avg_test_dice:.4f}")
    print(f"测试IoU: {avg_test_iou:.4f}")

if __name__ == "__main__":
    train_and_test()
