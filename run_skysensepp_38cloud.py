#!/usr/bin/env python3
"""
SkySense++ 38Cloud 训练和验证
完整工作流: 下载 -> 训练 -> 验证 -> 可视化
"""

import sys
import os
from pathlib import Path
import numpy as np
from tqdm import tqdm
import matplotlib.pyplot as plt

import torch
import torch.nn as nn
import torch.optim as optim
from torch.utils.data import Dataset, DataLoader, random_split
from PIL import Image

# 添加项目根目录
project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

class SimpleCloudDataset(Dataset):
    """简单的38Cloud数据集加载器"""
    
    def __init__(self, dataset_path, split='train', transform=None):
        self.dataset_path = Path(dataset_path)
        self.split = split
        self.transform = transform
        self.bands = ['blue', 'green', 'red', 'nir']
        
        # 查找训练目录
        self.train_dir = self._find_train_dir()
        
        # 获取所有patch
        self.patches = self._get_patches()
        print(f"{split}: 找到 {len(self.patches)} 个patches")
        
        # 划分训练和验证
        if len(self.patches) > 0:
            np.random.shuffle(self.patches)
            split_idx = int(0.8 * len(self.patches))
            if split == 'train':
                self.patches = self.patches[:split_idx]
            else:
                self.patches = self.patches[split_idx:]
        
        print(f"{split}: 使用 {len(self.patches)} 个patches")
    
    def _find_train_dir(self):
        possible_dirs = [
            self.dataset_path / "38-Cloud_training",
            self.dataset_path / "38-Cloud" / "38-Cloud_training",
            self.dataset_path,
        ]
        
        for dir_path in possible_dirs:
            if dir_path.exists() and len(list(dir_path.glob("train_blue"))) > 0:
                return dir_path
        
        # 递归搜索
        for item in self.dataset_path.rglob("train_blue"):
            return item.parent
        
        return self.dataset_path
    
    def _get_patches(self):
        patch_ids = set()
        
        # 查找蓝色波段文件
        blue_patterns = [
            self.train_dir / "train_blue" / "blue_patch_*.TIF",
            self.train_dir / "train_blue" / "blue_*.TIF",
            self.train_dir / "blue" / "blue_patch_*.TIF",
            self.train_dir / "blue" / "blue_*.TIF",
            self.train_dir / "*blue*.TIF"
        ]
        
        for pattern in blue_patterns:
            for f in self.train_dir.rglob(pattern.name):
                # 提取patch id
                filename = f.stem
                if filename.startswith("blue_patch_"):
                    patch_id = filename.replace("blue_patch_", "")
                elif filename.startswith("blue_"):
                    patch_id = filename.replace("blue_", "")
                else:
                    continue
                patch_ids.add(patch_id)
        
        return list(patch_ids)
    
    def _find_file(self, band, patch_id):
        possible_patterns = [
            f"train_{band}/{band}_patch_{patch_id}.TIF",
            f"train_{band}/{band}_{patch_id}.TIF",
            f"{band}/patch_{patch_id}.TIF",
            f"{band}/{band}_patch_{patch_id}.TIF",
        ]
        
        for pattern in possible_patterns:
            fpath = self.train_dir / pattern
            if fpath.exists():
                return fpath
        
        # 递归搜索
        search_pattern = f"*{band}*patch*{patch_id}*.TIF"
        for f in self.train_dir.rglob(search_pattern):
            return f
        
        # 简单搜索
        for f in self.train_dir.rglob(f"*{patch_id}*.TIF"):
            if band in f.name.lower():
                return f
        
        return None
    
    def _find_mask(self, patch_id):
        possible_patterns = [
            f"train_gt/gt_patch_{patch_id}.TIF",
            f"train_gt/gt_{patch_id}.TIF",
            f"gt/patch_{patch_id}.TIF",
            f"gt/gt_patch_{patch_id}.TIF",
        ]
        
        for pattern in possible_patterns:
            fpath = self.train_dir / pattern
            if fpath.exists():
                return fpath
        
        for f in self.train_dir.rglob(f"*gt*{patch_id}*.TIF"):
            return f
        
        for f in self.train_dir.rglob(f"*{patch_id}*.TIF"):
            if 'gt' in f.name.lower() or 'mask' in f.name.lower():
                return f
        
        return None
    
    def __len__(self):
        return len(self.patches)
    
    def __getitem__(self, idx):
        patch_id = self.patches[idx]
        
        # 加载波段
        bands = []
        for band in self.bands:
            band_file = self._find_file(band, patch_id)
            if band_file is not None:
                data = np.array(Image.open(band_file))
                bands.append(data)
            else:
                bands.append(np.zeros((384, 384), dtype=np.float32))
        
        # 堆叠波段
        if len(bands) > 0:
            image = np.stack(bands, axis=-1).astype(np.float32)
        else:
            image = np.zeros((384, 384, 4), dtype=np.float32)
        
        # 加载掩码
        mask_file = self._find_mask(patch_id)
        if mask_file is not None:
            mask = np.array(Image.open(mask_file))
            mask = (mask > 127).astype(np.float32)
        else:
            mask = np.zeros((384, 384), dtype=np.float32)
        
        # 归一化
        if image.max() > 1.0:
            image = image / 255.0
        
        # 转换为tensor
        image_tensor = torch.from_numpy(image).permute(2, 0, 1).float()
        mask_tensor = torch.from_numpy(mask).unsqueeze(0).float()
        
        # 确保形状一致
        if image_tensor.shape[1] != mask_tensor.shape[1]:
            import torch.nn.functional as F
            mask_tensor = F.interpolate(mask_tensor.unsqueeze(0), 
                                        size=image_tensor.shape[1:], 
                                        mode='nearest').squeeze(0)
        
        return image_tensor, mask_tensor

def download_dataset():
    """下载38Cloud数据集"""
    print("=" * 80)
    print("下载 38Cloud 数据集")
    print("=" * 80)
    
    try:
        import kagglehub
        path = kagglehub.dataset_download("sorour/38cloud-cloud-segmentation-in-satellite-images")
        print(f"✓ 数据集下载至: {path}")
        return Path(path)
    except Exception as e:
        print(f"✗ 使用kagglehub下载失败: {e}")
        print("尝试查找已有数据集...")
        
        possible_paths = [
            Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4"),
            Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images"),
            Path.cwd() / "data" / "38cloud",
            Path.cwd() / "38cloud",
        ]
        
        for p in possible_paths:
            if p.exists():
                print(f"✓ 找到已下载的数据集: {p}")
                return p
        
        print("✗ 未找到数据集，请手动下载")
        print("或检查配置路径")
        sys.exit(1)

def train_model():
    # 步骤1: 下载数据集
    dataset_path = download_dataset()
    
    # 步骤2: 导入SkySense++模型
    print("\n" + "=" * 80)
    print("导入 SkySense++ 模型")
    print("=" * 80)
    
    # 导入依赖
    from models.skysense_pp import SkySensePPModel
    
    # 步骤3: 创建数据加载器
    print("\n创建数据加载器...")
    train_dataset = SimpleCloudDataset(dataset_path, split='train')
    val_dataset = SimpleCloudDataset(dataset_path, split='val')
    
    if len(train_dataset) == 0:
        print("✗ 训练集为空！")
        print("尝试查找数据集...")
        
        # 打印目录结构
        print(f"\n数据集路径: {dataset_path}")
        print("内容:")
        try:
            for item in sorted(dataset_path.iterdir()):
                depth = len(item.relative_to(dataset_path).parts)
                indent = "  " * depth
                print(f"{indent}{'📁' if item.is_dir() else '📄'} {item.name}")
        except Exception as e:
            print(f"无法遍历: {e}")
        sys.exit(1)
    
    train_loader = DataLoader(train_dataset, batch_size=4, shuffle=True, num_workers=0)
    val_loader = DataLoader(val_dataset, batch_size=2, shuffle=False, num_workers=0)
    
    print(f"训练批次: {len(train_loader)}, 验证批次: {len(val_loader)}")
    
    # 步骤4: 创建模型
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f"\n使用设备: {device}")
    
    model = SkySensePPModel(
        encoder_name='resnet34',
        in_channels=4,
        num_classes=1,
        fusion_type='adaptive',
        use_semantic_enhancement=True,
        dropout=0.1,
        activation='sigmoid'
    ).to(device)
    
    # 统计参数量
    n_params = sum(p.numel() for p in model.parameters())
    print(f"SkySense++ 模型参数: {n_params:,}")
    
    # 步骤5: 损失函数和优化器
    criterion = nn.BCELoss()  # BCE损失用于二分类
    optimizer = optim.Adam(model.parameters(), lr=0.001)
    
    # 步骤6: 训练循环
    num_epochs = 5  # 快速训练演示
    best_dice = 0.0
    checkpoint_dir = project_root / "checkpoints"
    checkpoint_dir.mkdir(parents=True, exist_ok=True)
    
    train_losses = []
    val_dices = []
    
    print("\n" + "=" * 80)
    print("开始训练")
    print("=" * 80)
    
    for epoch in range(num_epochs):
        # 训练
        model.train()
        epoch_loss = 0.0
        
        pbar = tqdm(train_loader, desc=f"Epoch {epoch+1}/{num_epochs}")
        
        for images, masks in pbar:
            images = images.to(device)
            masks = masks.to(device)
            
            # 前向传播
            outputs = model(images)
            
            # 确保输出和目标形状一致
            if outputs.shape != masks.shape:
                import torch.nn.functional as F
                outputs = F.interpolate(outputs, size=masks.shape[2:], mode='bilinear', align_corners=True)
            
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
        
        # 验证
        print(f"\nEpoch {epoch+1}/{num_epochs} - 训练 Loss: {avg_train_loss:.4f}")
        print("验证中...")
        
        model.eval()
        val_dice_total = 0.0
        
        with torch.no_grad():
            for val_images, val_masks in val_loader:
                val_images = val_images.to(device)
                val_masks = val_masks.to(device)
                
                val_outputs = model(val_images)
                
                if val_outputs.shape != val_masks.shape:
                    import torch.nn.functional as F
                    val_outputs = F.interpolate(val_outputs, size=val_masks.shape[2:], mode='bilinear', align_corners=True)
                
                # 计算Dice
                pred_binary = (val_outputs > 0.5).float()
                intersection = (pred_binary * val_masks).sum()
                union = pred_binary.sum() + val_masks.sum()
                val_dice_total += (2.0 * intersection) / (union + 1e-7)
        
        avg_val_dice = val_dice_total / len(val_loader)
        val_dices.append(avg_val_dice.item())
        
        print(f"验证 Dice: {avg_val_dice:.4f}")
        
        # 保存最佳模型
        if avg_val_dice > best_dice:
            best_dice = avg_val_dice
            checkpoint_path = checkpoint_dir / "skysensepp_38cloud_best.pth"
            torch.save({
                'epoch': epoch + 1,
                'model_state_dict': model.state_dict(),
                'optimizer_state_dict': optimizer.state_dict(),
                'dice': best_dice,
            }, checkpoint_path)
            print(f"✓ 保存最佳模型至: {checkpoint_path}")
    
    print("\n" + "=" * 80)
    print("训练完成！")
    print(f"最佳验证 Dice: {best_dice:.4f}")
    
    # 步骤7: 生成可视化
    print("\n" + "=" * 80)
    print("生成验证结果可视化")
    print("=" * 80)
    
    output_dir = project_root / "38cloud_results"
    output_dir.mkdir(parents=True, exist_ok=True)
    
    # 加载最佳模型
    best_checkpoint = checkpoint_dir / "skysensepp_38cloud_best.pth"
    checkpoint = torch.load(best_checkpoint, map_location=device)
    model.load_state_dict(checkpoint['model_state_dict'])
    model.eval()
    
    # 生成可视化
    visualize_results(model, val_loader, device, output_dir)
    
    # 绘制训练曲线
    plot_curves(train_losses, val_dices, output_dir)
    
    print(f"\n✓ 结果已保存至: {output_dir}")
    
    return model, output_dir

def visualize_results(model, dataloader, device, output_dir, num_samples=10):
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
                import torch.nn.functional as F
                outputs = F.interpolate(outputs, size=masks.shape[2:], mode='bilinear', align_corners=True)
            
            # 转换为numpy
            images_np = images.cpu().numpy()
            masks_np = masks.cpu().numpy()
            outputs_np = outputs.cpu().numpy()
            
            for i in range(images_np.shape[0]):
                if samples_saved >= num_samples:
                    break
                
                fig, axes = plt.subplots(1, 4, figsize=(16, 4))
                
                # 输入RGB
                img_rgb = images_np[i, 1:4][::-1].transpose(1, 2, 0)  # R-G-B
                img_rgb = (img_rgb - img_rgb.min()) / (img_rgb.max() - img_rgb.min() + 1e-8)
                axes[0].imshow(img_rgb)
                axes[0].set_title("输入 (RGB)")
                axes[0].axis('off')
                
                # NIR
                img_nir = images_np[i, 3]
                axes[1].imshow(img_nir, cmap='gray')
                axes[1].set_title("近红外 (NIR)")
                axes[1].axis('off')
                
                # 真实掩码
                mask = masks_np[i, 0]
                axes[2].imshow(mask, cmap='gray')
                axes[2].set_title("真实掩码")
                axes[2].axis('off')
                
                # 预测
                pred = outputs_np[i, 0]
                pred_binary = (pred > 0.5).astype(np.float32)
                axes[3].imshow(pred_binary, cmap='gray')
                axes[3].set_title("预测结果")
                axes[3].axis('off')
                
                plt.tight_layout()
                save_path = output_dir / f"sample_{samples_saved+1:03d}.png"
                plt.savefig(save_path, dpi=150, bbox_inches='tight')
                plt.close()
                
                print(f"  保存 {save_path}")
                samples_saved += 1
            
            if samples_saved >= num_samples:
                break

def plot_curves(train_losses, val_dices, output_dir):
    """绘制训练曲线"""
    fig, axes = plt.subplots(1, 2, figsize=(12, 4))
    
    # Loss曲线
    axes[0].plot(range(1, len(train_losses)+1), train_losses, 'b-o', linewidth=2)
    axes[0].set_xlabel('Epoch', fontsize=11)
    axes[0].set_ylabel('Loss', fontsize=11)
    axes[0].set_title('训练损失', fontsize=12)
    axes[0].grid(True, alpha=0.3)
    
    # Dice曲线
    axes[1].plot(range(1, len(val_dices)+1), val_dices, 'r-o', linewidth=2, label='验证 Dice')
    axes[1].set_xlabel('Epoch', fontsize=11)
    axes[1].set_ylabel('Dice', fontsize=11)
    axes[1].set_title('验证 Dice 系数', fontsize=12)
    axes[1].legend()
    axes[1].grid(True, alpha=0.3)
    axes[1].set_ylim([0, 1])
    
    plt.tight_layout()
    save_path = output_dir / "training_curves.png"
    plt.savefig(save_path, dpi=150, bbox_inches='tight')
    plt.close()
    
    print(f"\n✓ 训练曲线保存至: {save_path}")

if __name__ == "__main__":
    train_model()
