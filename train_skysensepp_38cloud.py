#!/usr/bin/env python3
"""
SkySense++ 训练和验证脚本
下载38Cloud数据集，使用SkySense++模型进行训练，验证并输出可视化结果
"""

import sys
import os
from pathlib import Path
from tqdm import tqdm
import matplotlib.pyplot as plt
import numpy as np

import torch
import torch.nn as nn
import torch.optim as optim

# 添加项目根目录到路径
project_root = Path(__file__).parent
sys.path.insert(0, str(project_root))

def download_38cloud_dataset():
    """下载并准备38Cloud数据集"""
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
        print("尝试检查是否已存在...")
        
        # 检查可能的缓存位置
        possible_paths = [
            Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4"),
            Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/"),
            Path.cwd() / "data" / "38cloud"
        ]
        
        for p in possible_paths:
            if p.exists():
                print(f"✓ 找到已下载的数据集: {p}")
                return p
        
        print("✗ 未找到数据集，请手动下载")
        sys.exit(1)

def create_patch_csv(dataset_path):
    """创建训练和验证的patch列表"""
    data_dir = dataset_path / "38-Cloud_training" / "train_blue"
    
    if not data_dir.exists():
        print(f"✗ 找不到训练数据目录: {data_dir}")
        # 尝试查找
        print("搜索数据目录...")
        for root, dirs, files in os.walk(dataset_path):
            if "blue" in "".join(dirs + files).lower():
                data_dir = Path(root)
                break
    
    print(f"使用数据目录: {data_dir}")
    
    # 创建CSV文件
    csv_dir = project_root / "data" / "38cloud"
    csv_dir.mkdir(parents=True, exist_ok=True)
    
    patches = []
    for root, _, files in os.walk(dataset_path):
        for f in files:
            if f.startswith("blue_patch_") and f.endswith(".TIF"):
                patch_id = f.replace("blue_patch_", "").replace(".TIF", "")
                patches.append(patch_id)
    
    print(f"找到 {len(patches)} 个patches")
    
    # 划分训练和验证
    np.random.shuffle(patches)
    split_idx = int(len(patches) * 0.8)
    train_patches = patches[:split_idx]
    val_patches = patches[split_idx:]
    
    # 保存CSV
    pd = __import__('pandas')
    train_df = pd.DataFrame({'patch_id': train_patches})
    val_df = pd.DataFrame({'patch_id': val_patches})
    
    train_csv = csv_dir / "train_patches.csv"
    val_csv = csv_dir / "val_patches.csv"
    
    train_df.to_csv(train_csv, index=False)
    val_df.to_csv(val_csv, index=False)
    
    print(f"✓ 训练CSV保存至: {train_csv} ({len(train_patches)} patches)")
    print(f"✓ 验证CSV保存至: {val_csv} ({len(val_patches)} patches)")
    
    return str(csv_dir), len(train_patches), len(val_patches)

def main():
    print("\n" + "=" * 80)
    print("SkySense++ 云分割 - 训练与验证")
    print("=" * 80 + "\n")
    
    # 步骤1: 下载数据集
    dataset_path = download_38cloud_dataset()
    
    # 步骤2: 准备数据CSV
    csv_dir, train_count, val_count = create_patch_csv(dataset_path)
    
    # 步骤3: 导入项目模块
    from utils import load_config, set_seed, get_device, save_checkpoint, load_checkpoint, count_parameters
    from models.unet import create_model_from_config
    from models.losses import get_loss_function_from_config
    from models.metrics import SegmentationMetrics
    from data.multi_dataset_loader import get_combined_dataloader
    
    # 步骤4: 加载配置并修改为SkySense++
    config = load_config()
    
    # 确保使用SkySense++
    config.model['architecture'] = 'SkySensePP'
    config.model['fusion_type'] = 'adaptive'
    config.model['use_semantic_enhancement'] = True
    config.model['dropout'] = 0.1
    config.model['in_channels'] = 4
    config.model['out_channels'] = 1
    
    # 只使用38Cloud
    dataset_names = ['38cloud']
    
    # 步骤5: 设置训练参数
    set_seed(42)
    device = get_device()
    print(f"\n使用设备: {device}")
    
    # 步骤6: 创建数据加载器
    print("\n" + "=" * 80)
    print("创建数据加载器")
    print("=" * 80)
    
    # 临时更新配置中的数据集路径
    config.datasets['38cloud']['base_path'] = str(dataset_path)
    config.datasets['38cloud']['schedule_file'] = str(Path(csv_dir) / "train_patches.csv")
    config.datasets['38cloud']['weight'] = 1.0
    
    # 创建训练数据加载器
    print("\n训练数据加载器...")
    train_loader = get_combined_dataloader(
        dataset_names=dataset_names,
        strategy="weighted",
        batch_size=8,
        shuffle=True,
        num_workers=0,
        config=config,
        augment=True,
        target_channels=4
    )
    
    # 更新为验证CSV
    config.datasets['38cloud']['schedule_file'] = str(Path(csv_dir) / "val_patches.csv")
    
    # 创建验证数据加载器
    print("验证数据加载器...")
    val_loader = get_combined_dataloader(
        dataset_names=dataset_names,
        strategy="balanced",
        batch_size=4,
        shuffle=False,
        num_workers=0,
        config=config,
        augment=False,
        target_channels=4
    )
    
    print(f"\n训练样本: {len(train_loader.dataset)}, 批次: {len(train_loader)}")
    print(f"验证样本: {len(val_loader.dataset)}, 批次: {len(val_loader)}")
    
    # 步骤7: 创建模型
    print("\n" + "=" * 80)
    print("创建 SkySense++ 模型")
    print("=" * 80)
    model = create_model_from_config(config).to(device)
    print(f"✓ 模型参数总数: {count_parameters(model):,}")
    
    # 步骤8: 训练设置
    criterion = get_loss_function_from_config(config)
    optimizer = optim.Adam(
        model.parameters(),
        lr=config.training.get('learning_rate', 0.001),
        weight_decay=config.training.get('weight_decay', 1e-5)
    )
    
    num_epochs = 10  # 快速训练
    best_dice = 0.0
    checkpoint_dir = project_root / "checkpoints"
    checkpoint_dir.mkdir(parents=True, exist_ok=True)
    
    metrics = SegmentationMetrics()
    
    # 步骤9: 训练循环
    print("\n" + "=" * 80)
    print("开始训练")
    print("=" * 80)
    
    train_losses = []
    train_dices = []
    val_dices = []
    val_ious = []
    
    for epoch in range(num_epochs):
        model.train()
        epoch_loss = 0.0
        epoch_dice = 0.0
        
        pbar = tqdm(train_loader, desc=f"Epoch {epoch+1}/{num_epochs}", leave=False)
        
        for batch_idx, (images, masks) in enumerate(pbar):
            images = images.to(device)
            masks = masks.to(device)
            
            # 前向传播
            outputs = model(images)
            loss = criterion(outputs, masks)
            
            # 反向传播
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()
            
            epoch_loss += loss.item()
            batch_metrics = metrics(outputs, masks)
            epoch_dice += batch_metrics['dice']
            
            pbar.set_postfix({
                'loss': f'{loss.item():.4f}',
                'dice': f'{batch_metrics["dice"]:.4f}'
            })
        
        avg_loss = epoch_loss / len(train_loader)
        avg_train_dice = epoch_dice / len(train_loader)
        
        train_losses.append(avg_loss)
        train_dices.append(avg_train_dice)
        
        # 验证
        print(f"\nEpoch {epoch+1}/{num_epochs} - 训练 Loss: {avg_loss:.4f}, Dice: {avg_train_dice:.4f}")
        print("验证中...")
        
        model.eval()
        val_dice_total = 0.0
        val_iou_total = 0.0
        
        with torch.no_grad():
            for val_images, val_masks in val_loader:
                val_images = val_images.to(device)
                val_masks = val_masks.to(device)
                
                val_outputs = model(val_images)
                val_batch_metrics = metrics(val_outputs, val_masks)
                
                val_dice_total += val_batch_metrics['dice']
                val_iou_total += val_batch_metrics['iou']
        
        avg_val_dice = val_dice_total / len(val_loader)
        avg_val_iou = val_iou_total / len(val_loader)
        
        val_dices.append(avg_val_dice)
        val_ious.append(avg_val_iou)
        
        print(f"验证 - Dice: {avg_val_dice:.4f}, IoU: {avg_val_iou:.4f}")
        
        # 保存最佳模型
        if avg_val_dice > best_dice:
            best_dice = avg_val_dice
            checkpoint_path = checkpoint_dir / "skysensepp_best_model.pth"
            save_checkpoint(
                model=model,
                optimizer=optimizer,
                epoch=epoch+1,
                loss=avg_loss,
                checkpoint_dir=checkpoint_dir,
                filename="skysensepp_best_model.pth"
            )
            print(f"✓ 保存最佳模型 (Dice: {best_dice:.4f})")
    
    print("\n" + "=" * 80)
    print("训练完成！")
    print("=" * 80)
    print(f"最佳验证 Dice: {best_dice:.4f}")
    
    # 步骤10: 加载最佳模型并生成可视化
    print("\n" + "=" * 80)
    print("生成验证结果可视化")
    print("=" * 80)
    
    best_checkpoint = checkpoint_dir / "skysensepp_best_model.pth"
    model = create_model_from_config(config).to(device)
    load_checkpoint(model, best_checkpoint, device=device)
    model.eval()
    
    # 创建输出目录
    output_dir = project_root / "validation_results"
    output_dir.mkdir(parents=True, exist_ok=True)
    
    # 生成样本可视化
    visualize_samples(model, val_loader, device, output_dir, num_samples=10)
    
    # 绘制训练曲线
    plot_training_curves(train_losses, train_dices, val_dices, val_ious, output_dir)
    
    print(f"\n✓ 验证结果保存至: {output_dir}")
    print("\n任务完成！")

def visualize_samples(model, dataloader, device, output_dir, num_samples=10):
    """可视化模型预测结果"""
    model.eval()
    samples_saved = 0
    
    with torch.no_grad():
        for images, masks in dataloader:
            images = images.to(device)
            masks = masks.to(device)
            
            outputs = model(images)
            
            # 转换为numpy
            images_np = images.cpu().numpy()
            masks_np = masks.cpu().numpy()
            outputs_np = outputs.cpu().numpy()
            
            for i in range(images_np.shape[0]):
                if samples_saved >= num_samples:
                    break
                
                fig, axes = plt.subplots(1, 3, figsize=(15, 5))
                
                # 输入图像（使用RGB通道）
                img = images_np[i, :3].transpose(1, 2, 0)
                img = (img - img.min()) / (img.max() - img.min())  # 归一化到0-1
                axes[0].imshow(img)
                axes[0].set_title("输入 (RGB)")
                axes[0].axis('off')
                
                # 真实掩码
                mask = masks_np[i, 0]
                axes[1].imshow(mask, cmap='gray')
                axes[1].set_title("真实掩码")
                axes[1].axis('off')
                
                # 预测结果
                pred = outputs_np[i, 0]
                pred_binary = (pred > 0.5).astype(np.float32)
                axes[2].imshow(pred_binary, cmap='gray')
                axes[2].set_title("预测结果")
                axes[2].axis('off')
                
                plt.tight_layout()
                save_path = output_dir / f"result_{samples_saved+1:03d}.png"
                plt.savefig(save_path, dpi=150, bbox_inches='tight')
                plt.close()
                
                print(f"  保存 {save_path}")
                samples_saved += 1
            
            if samples_saved >= num_samples:
                break

def plot_training_curves(train_losses, train_dices, val_dices, val_ious, output_dir):
    """绘制训练曲线"""
    fig, axes = plt.subplots(1, 2, figsize=(14, 5))
    
    # Loss曲线
    axes[0].plot(range(1, len(train_losses)+1), train_losses, 'b-', label='训练Loss')
    axes[0].set_xlabel('Epoch')
    axes[0].set_ylabel('Loss')
    axes[0].set_title('训练损失')
    axes[0].legend()
    axes[0].grid(True, alpha=0.3)
    
    # Metric曲线
    axes[1].plot(range(1, len(train_dices)+1), train_dices, 'b-', label='训练Dice')
    axes[1].plot(range(1, len(val_dices)+1), val_dices, 'r-', label='验证Dice')
    axes[1].plot(range(1, len(val_ious)+1), val_ious, 'g--', label='验证IoU')
    axes[1].set_xlabel('Epoch')
    axes[1].set_ylabel('指标')
    axes[1].set_title('训练和验证指标')
    axes[1].legend()
    axes[1].grid(True, alpha=0.3)
    axes[1].set_ylim([0, 1])
    
    plt.tight_layout()
    save_path = output_dir / "training_curves.png"
    plt.savefig(save_path, dpi=150, bbox_inches='tight')
    plt.close()
    
    print(f"\n✓ 训练曲线保存至: {save_path}")

if __name__ == "__main__":
    main()
