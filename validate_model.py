"""
验证训练好的模型
"""

import torch
import numpy as np
from pathlib import Path
from PIL import Image
import matplotlib.pyplot as plt
from tqdm import tqdm

from models.unet import create_model
from models.metrics import SegmentationMetrics


def load_model(model_path, config):
    """加载训练好的模型"""
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    
    # 创建模型
    model = create_model(
        architecture=config['model'].get('architecture', 'Unet'),
        encoder_name=config['model'].get('encoder_name', 'resnet34'),
        encoder_weights=None,  # 不需要预训练权重
        in_channels=config['model'].get('in_channels', 4),
        classes=config['model'].get('out_channels', 1),
        activation='sigmoid'
    )
    
    # 加载权重
    model.load_state_dict(torch.load(model_path, map_location=device))
    model.to(device)
    model.eval()
    
    return model, device


def load_sample(name, base_path):
    """加载单个样本"""
    split_dir = Path(base_path) / "38-Cloud_training"
    
    # 加载多光谱波段
    blue_path = split_dir / "train_blue" / f"blue_{name}.TIF"
    green_path = split_dir / "train_green" / f"green_{name}.TIF"
    red_path = split_dir / "train_red" / f"red_{name}.TIF"
    nir_path = split_dir / "train_nir" / f"nir_{name}.TIF"
    gt_path = split_dir / "train_gt" / f"gt_{name}.TIF"
    
    # 读取图像
    blue = np.array(Image.open(blue_path))
    green = np.array(Image.open(green_path))
    red = np.array(Image.open(red_path))
    nir = np.array(Image.open(nir_path))
    
    # 组合成 4 通道图像
    image = np.stack([red, green, blue, nir], axis=-1)
    
    # 读取掩码
    mask = np.array(Image.open(gt_path))
    mask = (mask > 127).astype(np.float32)
    
    # 转换为 tensor
    image_tensor = torch.from_numpy(image).permute(2, 0, 1).float() / 65535.0
    mask_tensor = torch.from_numpy(mask).unsqueeze(0).float()
    
    return image, image_tensor, mask, mask_tensor


def validate_model(model, device, base_path, num_samples=5):
    """验证模型"""
    print("\n=== 开始验证 ===")
    
    # 查找有效样本
    gt_dir = Path(base_path) / "38-Cloud_training/train_gt"
    valid_names = []
    
    for gt_file in sorted(gt_dir.glob("*.TIF"))[:50]:  # 只检查前50个
        gt = np.array(Image.open(gt_file))
        if gt.max() > 0:
            name = gt_file.stem.replace("gt_", "")
            valid_names.append(name)
            if len(valid_names) >= num_samples:
                break
    
    print(f"找到 {len(valid_names)} 个有效样本进行验证\n")
    
    # 验证
    total_dice = 0
    total_iou = 0
    
    results = []
    
    for i, name in enumerate(valid_names, 1):
        print(f"验证样本 {i}/{len(valid_names)}: {name[:50]}...")
        
        # 加载样本
        image, image_tensor, mask, mask_tensor = load_sample(name, base_path)
        
        # 推理
        with torch.no_grad():
            image_tensor = image_tensor.unsqueeze(0).to(device)
            output = model(image_tensor)
        
        # 计算指标
        dice = SegmentationMetrics.dice_score(output, mask_tensor.unsqueeze(0).to(device))
        iou = SegmentationMetrics.iou_score(output, mask_tensor.unsqueeze(0).to(device))
        
        total_dice += dice
        total_iou += iou
        
        results.append({
            'name': name,
            'dice': dice,
            'iou': iou,
            'image': image,
            'mask': mask,
            'prediction': output.squeeze().cpu().numpy()
        })
        
        print(f"  Dice: {dice:.4f}, IoU: {iou:.4f}")
    
    # 计算平均值
    avg_dice = total_dice / len(valid_names)
    avg_iou = total_iou / len(valid_names)
    
    print(f"\n=== 验证结果 ===")
    print(f"平均 Dice Score: {avg_dice:.4f}")
    print(f"平均 IoU Score:  {avg_iou:.4f}")
    
    return results, avg_dice, avg_iou


def visualize_results(results, save_path='validation_results.png'):
    """可视化验证结果"""
    print("\n正在生成可视化结果...")
    
    n = len(results)
    fig, axes = plt.subplots(n, 4, figsize=(16, 4*n))
    
    if n == 1:
        axes = axes.reshape(1, -1)
    
    for i, result in enumerate(results):
        # 原始图像（RGB）
        rgb = result['image'][:, :, :3]
        rgb_normalized = (rgb - rgb.min()) / (rgb.max() - rgb.min() + 1e-8)
        
        axes[i, 0].imshow(rgb_normalized)
        axes[i, 0].set_title(f"RGB Image\n{result['name'][:30]}")
        axes[i, 0].axis('off')
        
        # 真实掩码
        axes[i, 1].imshow(result['mask'], cmap='gray')
        axes[i, 1].set_title(f"Ground Truth\nDice: {result['dice']:.3f}")
        axes[i, 1].axis('off')
        
        # 预测掩码（概率）
        pred_prob = result['prediction']
        axes[i, 2].imshow(pred_prob, cmap='hot')
        axes[i, 2].set_title("Prediction (Probability)")
        axes[i, 2].axis('off')
        
        # 预测掩码（二值化）
        pred_binary = (pred_prob > 0.5).astype(np.uint8)
        axes[i, 3].imshow(pred_binary, cmap='gray')
        axes[i, 3].set_title(f"Prediction (Binary)\nIoU: {result['iou']:.3f}")
        axes[i, 3].axis('off')
    
    plt.tight_layout()
    plt.savefig(save_path, dpi=150, bbox_inches='tight')
    print(f"可视化结果已保存到: {save_path}")
    plt.close()


if __name__ == "__main__":
    import yaml
    
    # 加载配置
    with open('config/config.yaml') as f:
        config = yaml.safe_load(f)
    
    # 模型路径
    model_path = 'checkpoints/best_model_005.pth'
    base_path = "/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4"
    
    # 检查模型是否存在
    if not Path(model_path).exists():
        print(f"错误: 模型文件 {model_path} 不存在!")
        print("请先运行训练脚本生成模型。")
        exit(1)
    
    # 加载模型
    print(f"加载模型: {model_path}")
    model, device = load_model(model_path, config)
    print(f"使用设备: {device}\n")
    
    # 验证模型
    results, avg_dice, avg_iou = validate_model(model, device, base_path, num_samples=5)
    
    # 可视化结果
    visualize_results(results, 'validation_results.png')
    
    print("\n验证完成！")
