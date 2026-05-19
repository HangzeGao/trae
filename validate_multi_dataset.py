"""
快速验证模型在多个数据集上的表现
支持 38-Cloud、95-Cloud 和 Cloud Cover Detection 数据集
"""
import yaml
import torch
import numpy as np
from pathlib import Path
from PIL import Image
import matplotlib.pyplot as plt
from typing import List, Dict, Tuple
import time

from models.unet import create_model


class MultiDatasetValidator:
    """多数据集验证器"""
    
    def __init__(self, model_path: str, config_path: str = "config/config.yaml"):
        # 加载配置
        with open(config_path) as f:
            self.config = yaml.safe_load(f)
        
        # 加载模型
        self.device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
        self.model = self._load_model(model_path)
        self.model.eval()
        
        # 数据集路径
        self.datasets = {
            '38cloud': {
                'name': '38-Cloud',
                'path': Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4"),
                'available': False
            },
            '95cloud': {
                'name': '95-Cloud',
                'path': Path("/root/.cache/kagglehub/datasets/sorour/95cloud-cloud-segmentation-on-satellite-images/versions/3"),
                'available': False
            },
            'cloud-cover': {
                'name': 'Cloud Cover Detection',
                'path': Path("/root/.cache/kagglehub/datasets/hmendonca/cloud-cover-detection"),
                'available': False
            }
        }
        
        # 检查数据集可用性
        self._check_dataset_availability()
    
    def _load_model(self, model_path: str):
        """加载模型"""
        model_cfg = self.config['model']
        model = create_model(
            architecture=model_cfg['architecture'],
            encoder_name=model_cfg['encoder_name'],
            encoder_weights=None,
            in_channels=model_cfg['in_channels'],
            classes=model_cfg['out_channels'],
            activation=model_cfg['activation']
        ).to(self.device)
        
        model.load_state_dict(torch.load(model_path, map_location=self.device))
        return model
    
    def _check_dataset_availability(self):
        """检查数据集可用性"""
        print("\n检查数据集可用性...")
        for key, dataset in self.datasets.items():
            if dataset['path'].exists():
                dataset['available'] = True
                print(f"  ✓ {dataset['name']}: 可用")
            else:
                print(f"  ✗ {dataset['name']}: 不可用")
    
    def _find_valid_samples(self, base_path: Path, num_samples: int = 5) -> List[str]:
        """查找有效样本"""
        gt_dir = base_path / "38-Cloud_training/train_gt"
        
        if not gt_dir.exists():
            # 尝试其他目录
            for subdir in ['train_gt', 'gt']:
                gt_dir = base_path / subdir
                if gt_dir.exists():
                    break
        
        if not gt_dir.exists():
            return []
        
        valid_samples = []
        for gt_file in sorted(gt_dir.glob('*.TIF')):
            gt = np.array(Image.open(gt_file))
            cloud_ratio = (gt > 0).mean()
            
            if 0.1 < cloud_ratio < 0.9:
                name = gt_file.stem.replace('gt_', '')
                valid_samples.append(name)
                if len(valid_samples) >= num_samples:
                    break
        
        return valid_samples
    
    def _load_sample(self, base_path: Path, sample_name: str) -> Tuple[np.ndarray, np.ndarray]:
        """加载单个样本"""
        split_dir = base_path / "38-Cloud_training"
        
        # 尝试不同的目录结构
        if not split_dir.exists():
            split_dir = base_path
        
        # 加载波段
        blue = np.array(Image.open(split_dir / f'train_blue/blue_{sample_name}.TIF'))
        green = np.array(Image.open(split_dir / f'train_green/green_{sample_name}.TIF'))
        red = np.array(Image.open(split_dir / f'train_red/red_{sample_name}.TIF'))
        nir = np.array(Image.open(split_dir / f'train_nir/nir_{sample_name}.TIF'))
        gt = np.array(Image.open(split_dir / f'train_gt/gt_{sample_name}.TIF'))
        
        # 组合图像
        image = np.stack([red, green, blue, nir], axis=-1)
        mask = (gt > 127).astype(np.float32)
        
        return image, mask
    
    def _dice_score(self, pred: np.ndarray, target: np.ndarray) -> float:
        """计算 Dice 得分"""
        pred_binary = (pred > 0.5).astype(np.float32)
        intersection = (pred_binary * target).sum()
        dice = (2. * intersection + 1e-7) / (pred_binary.sum() + target.sum() + 1e-7)
        return dice
    
    def _iou_score(self, pred: np.ndarray, target: np.ndarray) -> float:
        """计算 IoU 得分"""
        pred_binary = (pred > 0.5).astype(np.float32)
        intersection = (pred_binary * target).sum()
        union = pred_binary.sum() + target.sum() - intersection
        iou = (intersection + 1e-7) / (union + 1e-7)
        return iou
    
    def validate_dataset(self, dataset_key: str, num_samples: int = 5) -> Dict:
        """验证单个数据集"""
        if not self.datasets[dataset_key]['available']:
            return {'error': 'Dataset not available'}
        
        dataset = self.datasets[dataset_key]
        base_path = dataset['path']
        
        print(f"\n验证 {dataset['name']}...")
        
        # 查找有效样本
        sample_names = self._find_valid_samples(base_path, num_samples)
        
        if not sample_names:
            return {'error': 'No valid samples found'}
        
        print(f"  找到 {len(sample_names)} 个有效样本")
        
        # 验证
        total_dice = 0.0
        total_iou = 0.0
        results = []
        
        for i, name in enumerate(sample_names, 1):
            try:
                image, mask = self._load_sample(base_path, name)
                
                # 预测
                with torch.no_grad():
                    image_tensor = torch.from_numpy(image).permute(2, 0, 1).float() / 65535.0
                    image_tensor = image_tensor.unsqueeze(0).to(self.device)
                    output = self.model(image_tensor)
                    pred = output.squeeze().cpu().numpy()
                
                # 计算指标
                dice = self._dice_score(pred, mask)
                iou = self._iou_score(pred, mask)
                
                total_dice += dice
                total_iou += iou
                
                results.append({
                    'name': name,
                    'dice': dice,
                    'iou': iou,
                    'image': image,
                    'mask': mask,
                    'pred': pred
                })
                
                print(f"  样本 {i}: Dice={dice:.4f}, IoU={iou:.4f}")
                
            except Exception as e:
                print(f"  样本 {i}: 加载失败 - {str(e)}")
                continue
        
        if not results:
            return {'error': 'No samples validated successfully'}
        
        avg_dice = total_dice / len(results)
        avg_iou = total_iou / len(results)
        
        return {
            'dataset': dataset['name'],
            'num_samples': len(results),
            'avg_dice': avg_dice,
            'avg_iou': avg_iou,
            'samples': results
        }
    
    def validate_all(self, num_samples_per_dataset: int = 5) -> List[Dict]:
        """验证所有可用的数据集"""
        print("=" * 60)
        print("多数据集验证")
        print("=" * 60)
        
        available = [k for k, v in self.datasets.items() if v['available']]
        
        if not available:
            print("\n没有可用的数据集！")
            return []
        
        print(f"\n将验证以下数据集: {[self.datasets[k]['name'] for k in available]}")
        print(f"每个数据集使用 {num_samples_per_dataset} 个样本")
        
        all_results = []
        
        for dataset_key in available:
            result = self.validate_dataset(dataset_key, num_samples_per_dataset)
            if 'error' not in result:
                all_results.append(result)
            time.sleep(0.5)  # 避免输出过快
        
        return all_results
    
    def visualize_results(self, results: List[Dict], save_path: str = 'multi_dataset_validation.png'):
        """可视化验证结果"""
        if not results:
            return
        
        num_datasets = len(results)
        samples_per_dataset = min(3, results[0]['num_samples'])
        
        fig, axes = plt.subplots(
            num_datasets * samples_per_dataset, 4,
            figsize=(16, 4 * num_datasets * samples_per_dataset)
        )
        
        if num_datasets * samples_per_dataset == 1:
            axes = axes.reshape(1, -1)
        
        idx = 0
        for result in results:
            for i in range(min(samples_per_dataset, result['num_samples'])):
                sample = result['samples'][i]
                
                # RGB 图像
                rgb = sample['image'][:, :, :3].astype(float)
                rgb = (rgb - rgb.min()) / (rgb.max() - rgb.min() + 1e-8)
                
                axes[idx, 0].imshow(rgb)
                axes[idx, 0].set_title(f"{result['dataset'][:15]}\n{sample['name'][:30]}")
                axes[idx, 0].axis('off')
                
                # Ground Truth
                axes[idx, 1].imshow(sample['mask'], cmap='gray')
                axes[idx, 1].set_title('Ground Truth')
                axes[idx, 1].axis('off')
                
                # 预测概率
                axes[idx, 2].imshow(sample['pred'], cmap='hot')
                axes[idx, 2].set_title('Prediction')
                axes[idx, 2].axis('off')
                
                # 二值化预测
                axes[idx, 3].imshow((sample['pred'] > 0.5).astype(np.float32), cmap='gray')
                axes[idx, 3].set_title(f"Dice: {sample['dice']:.3f}\nIoU: {sample['iou']:.3f}")
                axes[idx, 3].axis('off')
                
                idx += 1
        
        plt.tight_layout()
        plt.savefig(save_path, dpi=150, bbox_inches='tight')
        print(f"\n✓ 可视化结果已保存到 {save_path}")
    
    def print_summary(self, results: List[Dict]):
        """打印总结"""
        print("\n" + "=" * 60)
        print("验证总结")
        print("=" * 60)
        
        for result in results:
            print(f"\n{result['dataset']}:")
            print(f"  样本数: {result['num_samples']}")
            print(f"  平均 Dice: {result['avg_dice']:.4f}")
            print(f"  平均 IoU:  {result['avg_iou']:.4f}")
        
        overall_dice = np.mean([r['avg_dice'] for r in results])
        overall_iou = np.mean([r['avg_iou'] for r in results])
        
        print(f"\n总体平均:")
        print(f"  平均 Dice: {overall_dice:.4f}")
        print(f"  平均 IoU:  {overall_iou:.4f}")
        print("=" * 60)


def main():
    """主函数"""
    # 模型路径
    model_path = 'checkpoints/best_model.pth'
    
    print("开始多数据集验证...")
    print(f"模型: {model_path}")
    
    # 创建验证器
    validator = MultiDatasetValidator(model_path)
    
    # 验证所有数据集
    results = validator.validate_all(num_samples_per_dataset=3)
    
    # 打印总结
    if results:
        validator.print_summary(results)
        
        # 可视化
        validator.visualize_results(results, 'multi_dataset_validation.png')
    else:
        print("\n没有数据集可供验证！")


if __name__ == "__main__":
    main()
