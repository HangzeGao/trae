"""
验证加权采样策略训练的模型
"""
import yaml
import torch
import numpy as np
from pathlib import Path
from PIL import Image
import matplotlib.pyplot as plt
from tqdm import tqdm

from models.unet import create_model
from models.metrics import SegmentationMetrics
from data.multi_dataset_loader import CloudSegmentationDataset


def validate_model(model_path='checkpoints/best_model_weighted.pth', num_samples=10):
    """验证模型"""
    
    # 加载配置
    with open('config/config.yaml') as f:
        config = yaml.safe_load(f)
    
    print("=" * 60)
    print("验证加权采样策略训练的模型")
    print("=" * 60)
    
    # 设备
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    print(f"设备: {device}")
    
    # 创建模型
    model_cfg = config['model']
    model = create_model(
        architecture=model_cfg['architecture'],
        encoder_name=model_cfg['encoder_name'],
        encoder_weights=None,
        in_channels=model_cfg['in_channels'],
        classes=model_cfg['out_channels'],
        activation=model_cfg['activation']
    ).to(device)
    
    # 加载模型权重
    if Path(model_path).exists():
        model.load_state_dict(torch.load(model_path, map_location=device))
        print(f"✓ 加载模型: {model_path}")
    else:
        print(f"✗ 模型文件不存在: {model_path}")
        return
    
    model.eval()
    
    # 创建验证数据集（使用38cloud的测试集）
    print("\n创建验证数据集...")
    val_dataset = CloudSegmentationDataset("38cloud", skip_invalid=True)
    
    # 随机选择验证样本
    np.random.seed(42)
    val_indices = np.random.choice(len(val_dataset), min(num_samples, len(val_dataset)), replace=False)
    
    print(f"验证样本数: {len(val_indices)}")
    
    # 验证
    all_dice = []
    all_iou = []
    results = []
    
    print("\n开始验证...")
    with torch.no_grad():
        for idx in tqdm(val_indices, desc="验证进度"):
            image, mask = val_dataset[idx]
            
            # 预测
            image_batch = image.unsqueeze(0).to(device)
            output = model(image_batch)
            pred = output.squeeze().cpu().numpy()
            
            # 计算指标
            mask_np = mask.squeeze().numpy()
            dice = SegmentationMetrics.dice_score(
                torch.from_numpy(pred).unsqueeze(0).unsqueeze(0),
                torch.from_numpy(mask_np).unsqueeze(0).unsqueeze(0)
            )
            iou = SegmentationMetrics.iou_score(
                torch.from_numpy(pred).unsqueeze(0).unsqueeze(0),
                torch.from_numpy(mask_np).unsqueeze(0).unsqueeze(0)
            )
            
            all_dice.append(dice)
            all_iou.append(iou)
            
            results.append({
                'dice': dice,
                'iou': iou,
                'image': image.numpy(),
                'mask': mask_np,
                'pred': pred
            })
    
    # 计算平均指标
    avg_dice = np.mean(all_dice)
    avg_iou = np.mean(all_iou)
    
    print("\n" + "=" * 60)
    print("验证结果")
    print("=" * 60)
    print(f"平均 Dice Score: {avg_dice:.4f}")
    print(f"平均 IoU Score:  {avg_iou:.4f}")
    
    # 显示每个样本的结果
    print(f"\n各样本结果:")
    for i, result in enumerate(results, 1):
        print(f"  样本 {i}: Dice={result['dice']:.4f}, IoU={result['iou']:.4f}")
    
    # 可视化部分结果
    visualize_results(results[:5], save_path='weighted_validation_results.png')
    
    print("\n" + "=" * 60)
    
    return {
        'avg_dice': avg_dice,
        'avg_iou': avg_iou,
        'results': results
    }


def visualize_results(results, save_path='validation_results.png'):
    """可视化验证结果"""
    num_samples = len(results)
    
    fig, axes = plt.subplots(num_samples, 4, figsize=(16, 4 * num_samples))
    
    if num_samples == 1:
        axes = axes.reshape(1, -1)
    
    for i, result in enumerate(results):
        # RGB 图像
        image = result['image']
        rgb = np.transpose(image[:3], (1, 2, 0))
        rgb = (rgb - rgb.min()) / (rgb.max() - rgb.min() + 1e-8)
        
        axes[i, 0].imshow(rgb)
        axes[i, 0].set_title(f'RGB Image')
        axes[i, 0].axis('off')
        
        # Ground Truth
        axes[i, 1].imshow(result['mask'], cmap='gray')
        axes[i, 1].set_title('Ground Truth')
        axes[i, 1].axis('off')
        
        # Prediction
        axes[i, 2].imshow(result['pred'], cmap='hot')
        axes[i, 2].set_title('Prediction (Prob)')
        axes[i, 2].axis('off')
        
        # Binary Prediction
        binary_pred = (result['pred'] > 0.5).astype(np.float32)
        axes[i, 3].imshow(binary_pred, cmap='gray')
        axes[i, 3].set_title(f'Binary\nDice: {result["dice"]:.3f}')
        axes[i, 3].axis('off')
    
    plt.tight_layout()
    plt.savefig(save_path, dpi=150, bbox_inches='tight')
    print(f"\n✓ 可视化结果已保存: {save_path}")


if __name__ == "__main__":
    # 尝试验证加权采样模型，如果不存在则验证默认模型
    model_path = 'checkpoints/best_model_weighted.pth'
    if not Path(model_path).exists():
        model_path = 'checkpoints/best_model.pth'
    
    validate_model(model_path=model_path, num_samples=10)
