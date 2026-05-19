"""
测试最佳模型，可视化预测结果
"""
import yaml
import torch
import numpy as np
from pathlib import Path
from PIL import Image
import matplotlib.pyplot as plt

from models.unet import create_model


def load_best_model(config_path, model_path):
    """加载最佳模型"""
    with open(config_path) as f:
        config = yaml.safe_load(f)
    
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    
    model = create_model(
        architecture=config['model']['architecture'],
        encoder_name=config['model']['encoder_name'],
        encoder_weights=None,
        in_channels=config['model']['in_channels'],
        classes=config['model']['out_channels'],
        activation=config['model']['activation'],
        **config['model'].get('model_kwargs', {})
    )
    
    model.load_state_dict(torch.load(model_path, map_location=device))
    model.to(device)
    model.eval()
    
    return model, device, config


def get_valid_samples(base_path, num_samples=5):
    """获取有效样本（云覆盖率适中）"""
    gt_dir = base_path / '38-Cloud_training/train_gt'
    
    valid_samples = []
    
    for gt_file in sorted(gt_dir.glob('*.TIF')):
        gt = np.array(Image.open(gt_file))
        cloud_ratio = (gt > 0).mean()
        
        # 只保留有一定云量，但又不是全云的样本
        if 0.1 < cloud_ratio < 0.9:
            name = gt_file.stem.replace('gt_', '')
            valid_samples.append(name)
            
            if len(valid_samples) >= num_samples:
                break
    
    return valid_samples


def load_single_sample(base_path, sample_name):
    """加载单个样本"""
    split_dir = base_path / '38-Cloud_training'
    
    blue = np.array(Image.open(split_dir / f'train_blue/blue_{sample_name}.TIF'))
    green = np.array(Image.open(split_dir / f'train_green/green_{sample_name}.TIF'))
    red = np.array(Image.open(split_dir / f'train_red/red_{sample_name}.TIF'))
    nir = np.array(Image.open(split_dir / f'train_nir/nir_{sample_name}.TIF'))
    gt = np.array(Image.open(split_dir / f'train_gt/gt_{sample_name}.TIF'))
    
    # 组合和归一化
    image = np.stack([red, green, blue, nir], axis=-1)
    image_tensor = torch.from_numpy(image).permute(2, 0, 1).float() / 65535.0
    
    mask = (gt > 127).astype(np.float32)
    
    return image, image_tensor, mask


def dice_score(pred, target, smooth=1e-7):
    """计算 Dice 得分"""
    pred_binary = (pred > 0.5).astype(np.float32)
    pred_flat = pred_binary.flatten()
    target_flat = target.flatten()
    
    intersection = (pred_flat * target_flat).sum()
    dice = (2. * intersection + smooth) / (pred_flat.sum() + target_flat.sum() + smooth)
    
    return dice


def visualize_predictions(model, device, base_path, sample_names, save_path='model_test_results.png'):
    """可视化预测结果"""
    num_samples = len(sample_names)
    
    fig, axes = plt.subplots(num_samples, 4, figsize=(16, 4 * num_samples))
    if num_samples == 1:
        axes = axes.reshape(1, -1)
    
    total_dice = 0.0
    
    for i, sample_name in enumerate(sample_names):
        image, image_tensor, mask = load_single_sample(base_path, sample_name)
        
        # 预测
        with torch.no_grad():
            image_input = image_tensor.unsqueeze(0).to(device)
            output = model(image_input)
            pred_proba = output.squeeze().cpu().numpy()
            pred_binary = (pred_proba > 0.5).astype(np.float32)
        
        # 计算指标
        dice = dice_score(pred_proba, mask)
        total_dice += dice
        
        # 显示 RGB 图
        rgb = image[:, :, :3].astype(float)
        rgb_normalized = (rgb - rgb.min()) / (rgb.max() - rgb.min() + 1e-8)
        
        axes[i, 0].imshow(rgb_normalized)
        axes[i, 0].set_title(f'Image\n{sample_name[:40]}')
        axes[i, 0].axis('off')
        
        # 显示真实标签
        axes[i, 1].imshow(mask, cmap='gray')
        axes[i, 1].set_title(f'Ground Truth\nCloud Ratio: {(mask.mean()*100):.1f}%')
        axes[i, 1].axis('off')
        
        # 显示预测概率
        axes[i, 2].imshow(pred_proba, cmap='hot')
        axes[i, 2].set_title(f'Prediction (Probability)')
        axes[i, 2].axis('off')
        
        # 显示二值化预测
        axes[i, 3].imshow(pred_binary, cmap='gray')
        axes[i, 3].set_title(f'Prediction (Binary)\nDice: {dice:.4f}')
        axes[i, 3].axis('off')
    
    avg_dice = total_dice / num_samples
    
    plt.suptitle(f'Cloud Segmentation Model Test\nAverage Dice Score: {avg_dice:.4f}', fontsize=16)
    plt.tight_layout()
    plt.savefig(save_path, dpi=150, bbox_inches='tight')
    print(f'✓ 可视化结果已保存到 {save_path}')
    
    return avg_dice


def main():
    # 配置路径
    config_path = 'config/config.yaml'
    model_path = 'checkpoints/best_model.pth'
    base_path = Path('/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4')
    
    print("=" * 60)
    print("测试最佳模型")
    print("=" * 60)
    
    # 加载模型
    print(f"\n加载模型: {model_path}")
    model, device, config = load_best_model(config_path, model_path)
    print(f"使用设备: {device}")
    print(f"模型参数: {sum(p.numel() for p in model.parameters()):,}")
    
    # 获取有效样本
    print(f"\n获取测试样本...")
    sample_names = get_valid_samples(base_path, num_samples=5)
    print(f"找到 {len(sample_names)} 个有效测试样本")
    
    # 可视化预测
    print(f"\n开始预测与可视化...")
    avg_dice = visualize_predictions(
        model, device, base_path, sample_names, 
        save_path='model_test_results.png'
    )
    
    print(f"\n测试完成！")
    print(f"平均 Dice Score: {avg_dice:.4f}")
    print("=" * 60)


if __name__ == "__main__":
    main()
