"""
快速验证训练好的模型
"""

import torch
import numpy as np
from pathlib import Path
from PIL import Image
import matplotlib.pyplot as plt

from models.unet import create_model
from models.metrics import SegmentationMetrics


def load_model(model_path, config):
    """加载训练好的模型"""
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    
    model = create_model(
        architecture=config['model'].get('architecture', 'Unet'),
        encoder_name=config['model'].get('encoder_name', 'resnet34'),
        encoder_weights=None,
        in_channels=config['model'].get('in_channels', 4),
        classes=config['model'].get('out_channels', 1),
        activation='sigmoid'
    )
    
    model.load_state_dict(torch.load(model_path, map_location=device))
    model.to(device)
    model.eval()
    
    return model, device


def validate_samples(model, device, base_path, num_samples=5):
    """验证模型"""
    print("\n=== 开始验证 ===")
    
    gt_dir = Path(base_path) / "38-Cloud_training/train_gt"
    valid_names = []
    
    for gt_file in sorted(gt_dir.glob("*.TIF"))[:50]:
        gt = np.array(Image.open(gt_file))
        if gt.max() > 0:
            name = gt_file.stem.replace("gt_", "")
            valid_names.append(name)
            if len(valid_names) >= num_samples:
                break
    
    print(f"验证 {len(valid_names)} 个样本\n")
    
    results = []
    total_dice = 0
    total_iou = 0
    
    for i, name in enumerate(valid_names, 1):
        # 加载样本
        split_dir = Path(base_path) / "38-Cloud_training"
        
        blue = np.array(Image.open(split_dir / "train_blue" / f"blue_{name}.TIF"))
        green = np.array(Image.open(split_dir / "train_green" / f"green_{name}.TIF"))
        red = np.array(Image.open(split_dir / "train_red" / f"red_{name}.TIF"))
        nir = np.array(Image.open(split_dir / "train_nir" / f"nir_{name}.TIF"))
        gt = np.array(Image.open(split_dir / "train_gt" / f"gt_{name}.TIF"))
        
        image = np.stack([red, green, blue, nir], axis=-1)
        mask = (gt > 127).astype(np.float32)
        
        image_tensor = torch.from_numpy(image).permute(2, 0, 1).float() / 65535.0
        mask_tensor = torch.from_numpy(mask).unsqueeze(0).float()
        
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
        
        print(f"样本 {i}: Dice={dice:.4f}, IoU={iou:.4f}")
    
    avg_dice = total_dice / len(valid_names)
    avg_iou = total_iou / len(valid_names)
    
    print(f"\n=== 平均结果 ===")
    print(f"Dice Score: {avg_dice:.4f}")
    print(f"IoU Score:  {avg_iou:.4f}")
    
    return results, avg_dice, avg_iou


def visualize(results, save_path='validation_results.png'):
    """可视化结果"""
    print(f"\n生成可视化结果: {save_path}")
    
    n = len(results)
    fig, axes = plt.subplots(n, 4, figsize=(16, 4*n))
    
    if n == 1:
        axes = axes.reshape(1, -1)
    
    for i, r in enumerate(results):
        rgb = r['image'][:, :, :3].astype(float)
        rgb = (rgb - rgb.min()) / (rgb.max() - rgb.min() + 1e-8)
        
        axes[i, 0].imshow(rgb)
        axes[i, 0].set_title(f"RGB: {r['name'][:40]}")
        axes[i, 0].axis('off')
        
        axes[i, 1].imshow(r['mask'], cmap='gray')
        axes[i, 1].set_title(f"Ground Truth\nDice: {r['dice']:.3f}")
        axes[i, 1].axis('off')
        
        axes[i, 2].imshow(r['prediction'], cmap='hot')
        axes[i, 2].set_title("Prediction (Prob)")
        axes[i, 2].axis('off')
        
        axes[i, 3].imshow((r['prediction'] > 0.5).astype(np.uint8), cmap='gray')
        axes[i, 3].set_title(f"Binary\nIoU: {r['iou']:.3f}")
        axes[i, 3].axis('off')
    
    plt.tight_layout()
    plt.savefig(save_path, dpi=150, bbox_inches='tight')
    print(f"✓ 已保存到 {save_path}")
    plt.close()


if __name__ == "__main__":
    import yaml
    
    with open('config/config.yaml') as f:
        config = yaml.safe_load(f)
    
    # 使用最佳模型
    model_path = 'checkpoints/best_model_004.pth'
    base_path = "/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4"
    
    print(f"加载模型: {model_path}")
    model, device = load_model(model_path, config)
    print(f"设备: {device}\n")
    
    results, avg_dice, avg_iou = validate_samples(model, device, base_path, num_samples=5)
    visualize(results, 'validation_results.png')
    
    print("\n验证完成！")
