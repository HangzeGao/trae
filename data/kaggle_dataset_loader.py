"""
优雅的 Kaggle 云分割数据集加载器
支持多个云分割数据集的统一接口
"""

import pandas as pd
from pathlib import Path
from typing import Dict, List, Optional, Tuple, Any
import numpy as np
from PIL import Image
import torch
from torch.utils.data import Dataset, DataLoader
from dataclasses import dataclass, field


@dataclass
class DatasetConfig:
    """数据集配置"""
    name: str
    base_path: Path
    schedule_file: str
    bands: List[str] = field(default_factory=lambda: ['red', 'green', 'blue', 'nir'])
    mask_suffix: str = "gt"
    mask_threshold: int = 127
    file_extension: str = ".TIF"
    
    def get_band_path(self, band: str, filename: str, prefix: str = "") -> Path:
        """获取波段文件路径"""
        band_name = f"{prefix}{band}" if prefix else band
        return self.base_path / f"{band_name}/{band_name}_{filename}{self.file_extension}"
    
    def get_mask_path(self, filename: str, prefix: str = "") -> Path:
        """获取掩码文件路径"""
        mask_name = f"{prefix}{self.mask_suffix}" if prefix else self.mask_suffix
        return self.base_path / f"{mask_name}/{mask_name}_{filename}{self.file_extension}"


# 数据集注册表
DATASET_REGISTRY: Dict[str, DatasetConfig] = {
    "38cloud": DatasetConfig(
        name="38-Cloud",
        base_path=Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4"),
        schedule_file="/workspace/data/38cloud/train_patches.csv",
        bands=['blue', 'green', 'red', 'nir'],
        mask_suffix="gt",
        file_extension=".TIF"
    ),
    "95cloud": DatasetConfig(
        name="95-Cloud",
        base_path=Path("/root/.cache/kagglehub/datasets/sorour/95cloud-cloud-segmentation-on-satellite-images/versions/3"),
        schedule_file="/workspace/data/95cloud/train_patches.csv",
        bands=['blue', 'green', 'red', 'nir'],
        mask_suffix="gt",
        file_extension=".TIF"
    ),
    "cloud-cover": DatasetConfig(
        name="Cloud Cover Detection",
        base_path=Path("/root/.cache/kagglehub/datasets/hmendonca/cloud-cover-detection"),
        schedule_file="/workspace/data/cloud-cover/train_patches.csv",
        bands=['red', 'green', 'blue'],  # 可能需要调整
        mask_suffix="mask",
        file_extension=".png"
    )
}


class CloudSegmentationDataset(Dataset):
    """
    统一的云分割数据集加载器
    
    支持的数据集:
    - 38cloud: 38-Cloud 云分割数据集
    - 95cloud: 95-Cloud 云分割数据集
    - cloud-cover: Cloud Cover Detection 数据集
    """
    
    def __init__(
        self,
        dataset_name: str,
        schedule_path: Optional[str] = None,
        transform=None,
        normalize: bool = True,
        mask_threshold: Optional[int] = None
    ):
        """
        Args:
            dataset_name: 数据集名称 ('38cloud', '95cloud', 'cloud-cover')
            schedule_path: 调度文件路径，如果为 None 则使用注册表中的默认路径
            transform: 数据变换
            normalize: 是否归一化图像
            mask_threshold: 掩码二值化阈值
        """
        self.dataset_name = dataset_name
        self.transform = transform
        self.normalize = normalize
        
        # 获取数据集配置
        if dataset_name not in DATASET_REGISTRY:
            raise ValueError(f"未知的数据集: {dataset_name}. 可用数据集: {list(DATASET_REGISTRY.keys())}")
        
        self.config = DATASET_REGISTRY[dataset_name]
        
        # 使用提供的路径或默认路径
        self.schedule_path = schedule_path or self.config.schedule_file
        self.schedule_df = pd.read_csv(self.schedule_path)
        
        # 掩码阈值
        self.mask_threshold = mask_threshold if mask_threshold is not None else self.config.mask_threshold
        
        # 验证数据集路径
        if not self.config.base_path.exists():
            raise FileNotFoundError(f"数据集路径不存在: {self.config.base_path}")
    
    def __len__(self) -> int:
        return len(self.schedule_df)
    
    def __getitem__(self, idx: int) -> Tuple[torch.Tensor, torch.Tensor]:
        """获取单个样本"""
        name = self.schedule_df.iloc[idx]['name']
        
        # 构建图像路径
        image_bands = []
        for band in self.config.bands:
            band_path = self._get_band_path(band, name)
            band_data = np.array(Image.open(band_path))
            image_bands.append(band_data)
        
        # 组合多光谱图像 (H, W, C)
        image = np.stack(image_bands, axis=-1)
        
        # 获取掩码
        mask_path = self._get_mask_path(name)
        mask = np.array(Image.open(mask_path))
        mask = (mask > self.mask_threshold).astype(np.float32)
        
        # 归一化
        if self.normalize:
            image = image.astype(np.float32) / 65535.0 if image.max() > 1 else image.astype(np.float32) / 255.0
        
        # 转换为 PyTorch 张量 (C, H, W)
        image_tensor = torch.from_numpy(image).permute(2, 0, 1).float()
        mask_tensor = torch.from_numpy(mask).unsqueeze(0).float()
        
        # 应用变换
        if self.transform:
            image_tensor = self.transform(image_tensor)
            mask_tensor = self.transform(mask_tensor)
        
        return image_tensor, mask_tensor
    
    def _get_band_path(self, band: str, name: str) -> Path:
        """获取波段文件路径"""
        band_path = self.config.base_path / f"train_{band}" / f"train_{band}_{name}{self.config.file_extension}"
        if band_path.exists():
            return band_path
        
        # 备选路径格式
        band_path = self.config.base_path / f"{band}" / f"{band}_{name}{self.config.file_extension}"
        if band_path.exists():
            return band_path
        
        raise FileNotFoundError(f"找不到波段文件: train_{band}/{band}_{name}{self.config.file_extension}")
    
    def _get_mask_path(self, name: str) -> Path:
        """获取掩码文件路径"""
        mask_path = self.config.base_path / f"train_gt" / f"train_gt_{name}{self.config.file_extension}"
        if mask_path.exists():
            return mask_path
        
        # 备选路径格式
        mask_path = self.config.base_path / f"gt" / f"gt_{name}{self.config.file_extension}"
        if mask_path.exists():
            return mask_path
        
        raise FileNotFoundError(f"找不到掩码文件: gt/gt_{name}{self.config.file_extension}")


class MultiDatasetWrapper(Dataset):
    """多数据集包装器，用于合并多个数据集"""
    
    def __init__(self, datasets: List[CloudSegmentationDataset]):
        self.datasets = datasets
        self.cumulative_sizes = self._calculate_cumulative_sizes()
    
    def _calculate_cumulative_sizes(self) -> List[int]:
        sizes = [len(d) for d in self.datasets]
        cumulative = []
        total = 0
        for size in sizes:
            cumulative.append(total)
            total += size
        cumulative.append(total)
        return cumulative
    
    def __len__(self) -> int:
        return self.cumulative_sizes[-1]
    
    def __getitem__(self, idx: int) -> Tuple[torch.Tensor, torch.Tensor]:
        dataset_idx = self._find_dataset_idx(idx)
        sample_idx = idx - self.cumulative_sizes[dataset_idx]
        return self.datasets[dataset_idx][sample_idx]
    
    def _find_dataset_idx(self, idx: int) -> int:
        for i in range(len(self.cumulative_sizes) - 1):
            if self.cumulative_sizes[i] <= idx < self.cumulative_sizes[i + 1]:
                return i
        return len(self.datasets) - 1


def create_dataloader(
    dataset_name: str,
    batch_size: int = 8,
    shuffle: bool = True,
    num_workers: int = 0,
    schedule_path: Optional[str] = None,
    **dataset_kwargs
) -> DataLoader:
    """
    创建单个数据集的 DataLoader
    
    Args:
        dataset_name: 数据集名称
        batch_size: 批次大小
        shuffle: 是否打乱
        num_workers: 数据加载线程数
        schedule_path: 调度文件路径
        **dataset_kwargs: 传递给 CloudSegmentationDataset 的其他参数
    
    Returns:
        DataLoader 对象
    """
    dataset = CloudSegmentationDataset(
        dataset_name=dataset_name,
        schedule_path=schedule_path,
        **dataset_kwargs
    )
    
    return DataLoader(
        dataset,
        batch_size=batch_size,
        shuffle=shuffle,
        num_workers=num_workers
    )


def create_combined_dataloader(
    dataset_names: List[str],
    batch_size: int = 8,
    shuffle: bool = True,
    num_workers: int = 0,
    schedule_paths: Optional[Dict[str, str]] = None,
    **dataset_kwargs
) -> DataLoader:
    """
    创建合并多个数据集的 DataLoader
    
    Args:
        dataset_names: 数据集名称列表
        batch_size: 批次大小
        shuffle: 是否打乱
        num_workers: 数据加载线程数
        schedule_paths: 数据集名称到调度文件路径的映射
        **dataset_kwargs: 传递给 CloudSegmentationDataset 的其他参数
    
    Returns:
        DataLoader 对象
    """
    paths = schedule_paths or {}
    datasets = [
        CloudSegmentationDataset(
            dataset_name=name,
            schedule_path=paths.get(name),
            **dataset_kwargs
        )
        for name in dataset_names
    ]
    
    combined_dataset = MultiDatasetWrapper(datasets)
    
    return DataLoader(
        combined_dataset,
        batch_size=batch_size,
        shuffle=shuffle,
        num_workers=num_workers
    )


def get_dataset_info(dataset_name: str) -> Dict[str, Any]:
    """获取数据集信息"""
    if dataset_name not in DATASET_REGISTRY:
        raise ValueError(f"未知的数据集: {dataset_name}")
    
    config = DATASET_REGISTRY[dataset_name]
    schedule_path = Path(config.schedule_file)
    
    info = {
        "name": config.name,
        "dataset_key": dataset_name,
        "base_path": str(config.base_path),
        "schedule_file": str(schedule_path),
        "bands": config.bands,
        "mask_suffix": config.mask_suffix,
        "file_extension": config.file_extension,
    }
    
    if schedule_path.exists():
        df = pd.read_csv(schedule_path)
        info["num_samples"] = len(df)
    else:
        info["num_samples"] = "N/A (文件不存在)"
    
    return info


def list_available_datasets() -> List[Dict[str, Any]]:
    """列出所有可用的数据集"""
    return [
        {
            "key": key,
            "name": config.name,
            "base_path": str(config.base_path),
            "available": config.base_path.exists()
        }
        for key, config in DATASET_REGISTRY.items()
    ]


# 示例用法
if __name__ == "__main__":
    print("=" * 60)
    print("Kaggle Cloud Segmentation Dataset Loader")
    print("=" * 60)
    
    # 列出所有数据集
    print("\n可用数据集:")
    for info in list_available_datasets():
        status = "✓" if info["available"] else "✗"
        print(f"  {status} [{info['key']}] {info['name']}")
        print(f"     路径: {info['base_path']}")
    
    # 获取数据集信息
    print("\n数据集详情:")
    for dataset_key in ["38cloud", "95cloud"]:
        info = get_dataset_info(dataset_key)
        print(f"\n  {info['name']}:")
        print(f"    样本数: {info['num_samples']}")
        print(f"    波段: {info['bands']}")
        print(f"    文件格式: {info['file_extension']}")
    
    print("\n" + "=" * 60)
