"""
增强版多数据集验证
详细对比三个数据集上的模型表现
"""
import yaml
import torch
import numpy as np
from pathlib import Path
from PIL import Image
import matplotlib.pyplot as plt
import pandas as pd
from typing import List, Dict, Optional

from models.unet import create_model


class EnhancedValidator:
    """增强版验证器"""
    
    def __init__(self, model_path: str = "checkpoints/best_model.pth"):
        # 加载配置
        with open('config/config.yaml') as f:
            self.config = yaml.safe_load(f)
        
        # 加载模型
        self.device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
        self.model = self._load_model(model_path)
        
        # 数据集配置
        self.datasets = {
            '38cloud': {
                'name': '38-Cloud',
                'path': Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4"),
                'train_dir': '38-Cloud_training',
                'bands': ['blue', 'green', 'red', 'nir'],
                'mask_prefix': 'gt',
                'available': False
            },
            '95cloud': {
                'name': '95-Cloud',
                'path': Path("/root/.cache/kagglehub/datasets/sorour/95cloud-cloud-segmentation-on-satellite-images/versions/3"),
                'train_dir': '95-cloud_training_only_additional_to38-cloud',
                'bands': ['blue', 'green', 'red', 'nir'],
                'mask_prefix': 'gt',
                'available': False
            },
            'cloud-cover': {
                'name': 'Cloud Cover Detection',
                'path': Path("/root/.cache/kagglehub/datasets/hmendonca/cloud-cover-detection"),
                'train_dir': '',
                'bands': ['red', 'green', 'blue'],
                'mask_prefix': 'mask',
                'available': False
            }
        }
        
        self._check_availability()
    
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
        model.eval()
        return model
    
    def _check_availability(self):
        """检查数据集可用性"""
        print("\n检查数据集可用性...")
        for key, ds in self.datasets.items():
            if ds['path'].exists():
                if self._has_valid_data(ds['path']):
                    ds['available'] = True
                    print(f"  ✓ {ds['name']}: 可用")
                else:
                    print(f"  ✗ {ds['name']}: 目录存在但无有效数据")
            else:
                print(f"  ✗ {ds['name']}: 未找到")
    
    def _has_valid_data(self, base_path: Path) -> bool:
        """检查是否有有效数据"""
        for pattern in ['**/*.TIF', '**/*.tif', '**/*.png']:
            files = list(base_path.glob(pattern))[:3]
            if files:
                return True
        return False
    
    def _find_samples_38cloud(self, base_path: Path, num: int) -> List[Dict]:
        """查找38cloud样本"""
        train_path = base_path / self.datasets['38cloud']['train_dir']
        gt_dir = train_path / 'train_gt'
        
        samples = []
        for gt_file in sorted(gt_dir.glob('*.TIF'))[:100]:
            gt = np.array(Image.open(gt_file))
            ratio = (gt > 0).mean()
            
            if 0.1 < ratio < 0.9:
                name = gt_file.stem.replace('gt_', '')
                samples.append({'name': name, 'ratio': ratio, 'gt_path': gt_file})
                if len(samples) >= num:
                    break
        
        return samples
    
    def _find_samples_95cloud(self, base_path: Path, num: int) -> List[Dict]:
        """查找95cloud样本"""
        train_path = base_path / self.datasets['95cloud']['train_dir']
        
        for subdir in ['train_gt_additional_to38cloud', 'gt', 'train_gt']:
            gt_dir = train_path / subdir
            if not gt_dir.exists():
                continue
            
            samples = []
            for gt_file in sorted(gt_dir.glob('*.TIF'))[:100]:
                gt = np.array(Image.open(gt_file))
                ratio = (gt > 0).mean()
                
                if 0.1 < ratio < 0.9:
                    name = gt_file.stem.replace(f'{subdir.split("_")[0]}_', '')
                    samples.append({'name': name, 'ratio': ratio, 'gt_path': gt_file, 'subdir': subdir})
                    if len(samples) >= num:
                        break
            
            if samples:
                return samples
        
        return []
    
    def _validate_single_sample(self, base_path: Path, sample: Dict, ds_key: str) -> Optional[Dict]:
        """验证单个样本"""
        try:
            if ds_key == '38cloud':
                train_path = base_path / self.datasets[ds_key]['train_dir']
                name = sample['name']
                
                blue = np.array(Image.open(train_path / f'train_blue/blue_{name}.TIF'))
                green = np.array(Image.open(train_path / f'train_green/green_{name}.TIF'))
                red = np.array(Image.open(train_path / f'train_red/red_{name}.TIF'))
                nir = np.array(Image.open(train_path / f'train_nir/nir_{name}.TIF'))
                gt = np.array(Image.open(sample['gt_path']))
                
                image = np.stack([red, green, blue, nir], axis=-1)
                mask = (gt > 127).astype(np.float32)
                
            elif ds_key == '95cloud':
                train_path = base_path / self.datasets[ds_key]['train_dir']
                name = sample['name']
                
                blue = np.array(Image.open(train_path / f'train_blue_additional_to38cloud/blue_{name}.TIF'))
                green = np.array(Image.open(train_path / f'train_green_additional_to38cloud/green_{name}.TIF'))
                red = np.array(Image.open(train_path / f'train_red_additional_to38cloud/red_{name}.TIF'))
                nir = np.array(Image.open(train_path / f'train_nir_additional_to38cloud/nir_{name}.TIF'))
                gt = np.array(Image.open(sample['gt_path']))
                
                image = np.stack([red, green, blue, nir], axis=-1)
                mask = (gt > 127).astype(np.float32)
            
            else:
                return None
            
            with torch.no_grad():
                image_tensor = torch.from_numpy(image).permute(2, 0, 1).float()
                if image_tensor.max() > 1:
                    image_tensor = image_tensor / 65535.0
                image_tensor = image_tensor.unsqueeze(0).to(self.device)
                output = self.model(image_tensor)
                pred = output.squeeze().cpu().numpy()
            
            dice = self._dice(pred, mask)
            iou = self._iou(pred, mask)
            
            return {
                'name': sample['name'],
                'image': image,
                'mask': mask,
                'pred': pred,
                'dice': dice,
                'iou': iou
            }
            
        except Exception as e:
            print(f"    加载失败: {str(e)[:50]}")
            return None
    
    def _dice(self, pred: np.ndarray, target: np.ndarray) -> float:
        pred_bin = (pred > 0.5).astype(np.float32)
        intersection = (pred_bin * target).sum()
        return (2. * intersection + 1e-7) / (pred_bin.sum() + target.sum() + 1e-7)
    
    def _iou(self, pred: np.ndarray, target: np.ndarray) -> float:
        pred_bin = (pred > 0.5).astype(np.float32)
        intersection = (pred_bin * target).sum()
        union = pred_bin.sum() + target.sum() - intersection
        return (intersection + 1e-7) / (union + 1e-7)
    
    def validate_dataset(self, ds_key: str, num_samples: int = 5) -> Optional[Dict]:
        """验证单个数据集"""
        ds = self.datasets[ds_key]
        
        if not ds['available']:
            return None
        
        print(f"\n验证 {ds['name']}...")
        
        if ds_key == '38cloud':
            samples = self._find_samples_38cloud(ds['path'], num_samples)
        elif ds_key == '95cloud':
            samples = self._find_samples_95cloud(ds['path'], num_samples)
        else:
            return None
        
        if not samples:
            print(f"  未找到有效样本")
            return None
        
        print(f"  找到 {len(samples)} 个有效样本")
        
        results = []
        total_dice = 0.0
        total_iou = 0.0
        
        for i, sample in enumerate(samples, 1):
            result = self._validate_single_sample(ds['path'], sample, ds_key)
            
            if result:
                total_dice += result['dice']
                total_iou += result['iou']
                results.append(result)
                print(f"  样本 {i}: Dice={result['dice']:.4f}, IoU={result['iou']:.4f}")
        
        if not results:
            return None
        
        return {
            'dataset': ds['name'],
            'dataset_key': ds_key,
            'num_samples': len(results),
            'avg_dice': total_dice / len(results),
            'avg_iou': total_iou / len(results),
            'samples': results
        }
    
    def validate_all(self, num_per_dataset: int = 5) -> List[Dict]:
        """验证所有数据集"""
        print("=" * 70)
        print("增强版多数据集验证")
        print("=" * 70)
        print(f"模型: checkpoints/best_model.pth")
        print(f"设备: {self.device}")
        
        all_results = []
        
        for ds_key in self.datasets.keys():
            result = self.validate_dataset(ds_key, num_per_dataset)
            if result:
                all_results.append(result)
        
        return all_results
    
    def print_summary(self, results: List[Dict]):
        """打印总结"""
        print("\n" + "=" * 70)
        print("验证总结")
        print("=" * 70)
        
        for result in results:
            print(f"\n{result['dataset']}:")
            print(f"  验证样本数: {result['num_samples']}")
            print(f"  平均 Dice:  {result['avg_dice']:.4f}")
            print(f"  平均 IoU:   {result['avg_iou']:.4f}")
        
        if results:
            overall_dice = np.mean([r['avg_dice'] for r in results])
            overall_iou = np.mean([r['avg_iou'] for r in results])
            
            print(f"\n总体性能:")
            print(f"  平均 Dice:  {overall_dice:.4f}")
            print(f"  平均 IoU:   {overall_iou:.4f}")
            print(f"  验证数据集数: {len(results)}")
        
        print("=" * 70)
    
    def create_comparison_table(self, results: List[Dict], save_path: str = 'validation_comparison.csv'):
        """创建对比表格"""
        if not results:
            return
        
        data = []
        for result in results:
            data.append({
                '数据集': result['dataset'],
                '样本数': result['num_samples'],
                '平均 Dice': f"{result['avg_dice']:.4f}",
                '平均 IoU': f"{result['avg_iou']:.4f}"
            })
        
        df = pd.DataFrame(data)
        df.to_csv(save_path, index=False, encoding='utf-8-sig')
        print(f"\n✓ 对比表格已保存到 {save_path}")
    
    def visualize_comparison(self, results: List[Dict], save_path: str = 'dataset_comparison.png'):
        """可视化对比"""
        if not results:
            return
        
        fig, axes = plt.subplots(1, 2, figsize=(12, 5))
        
        datasets = [r['dataset'] for r in results]
        dice_scores = [r['avg_dice'] for r in results]
        iou_scores = [r['avg_iou'] for r in results]
        
        colors = ['#3498db', '#e74c3c', '#2ecc71'][:len(datasets)]
        
        axes[0].bar(datasets, dice_scores, color=colors)
        axes[0].set_ylabel('Dice Score')
        axes[0].set_title('Dice Score Comparison')
        axes[0].set_ylim(0, 1)
        for i, v in enumerate(dice_scores):
            axes[0].text(i, v + 0.02, f'{v:.4f}', ha='center', fontsize=10)
        
        axes[1].bar(datasets, iou_scores, color=colors)
        axes[1].set_ylabel('IoU Score')
        axes[1].set_title('IoU Score Comparison')
        axes[1].set_ylim(0, 1)
        for i, v in enumerate(iou_scores):
            axes[1].text(i, v + 0.02, f'{v:.4f}', ha='center', fontsize=10)
        
        plt.tight_layout()
        plt.savefig(save_path, dpi=150, bbox_inches='tight')
        print(f"✓ 对比图已保存到 {save_path}")


def main():
    """主函数"""
    validator = EnhancedValidator()
    results = validator.validate_all(num_per_dataset=5)
    
    if results:
        validator.print_summary(results)
        validator.create_comparison_table(results)
        validator.visualize_comparison(results)
    else:
        print("\n没有数据集可供验证！")


if __name__ == "__main__":
    main()
