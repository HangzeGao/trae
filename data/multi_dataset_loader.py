"""
多数据集加载器 - 支持多种合并策略
支持不同结构数据集的统一接口
"""
import pandas as pd
from pathlib import Path
from typing import Dict, List, Optional, Tuple, Any, Union
import numpy as np
from PIL import Image
import torch
from torch.utils.data import Dataset, DataLoader, ConcatDataset, WeightedRandomSampler, Subset
import random

try:
    from utils.config import Config, DatasetConfig, MergeStrategy
    HAS_CONFIG = True
except ImportError:
    HAS_CONFIG = False
    from dataclasses import dataclass, field
    from enum import Enum
    
    class MergeStrategy(Enum):
        """数据集合并策略"""
        CONCAT = "concat"
        WEIGHTED = "weighted"
        ROUND_ROBIN = "round_robin"
        BALANCED = "balanced"
        SAMPLE = "sample"
    
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
        weight: float = 1.0
        train_dir: Optional[str] = None


# Fallback registry if config is not available
DEFAULT_DATASET_REGISTRY: Dict[str, DatasetConfig] = {
    "38cloud": DatasetConfig(
        name="38-Cloud",
        base_path=Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4"),
        schedule_file="./data/38cloud/train_patches.csv",
        bands=['blue', 'green', 'red', 'nir'],
        mask_suffix="gt",
        file_extension=".TIF",
        weight=1.0,
        train_dir="38-Cloud_training"
    ),
    "95cloud": DatasetConfig(
        name="95-Cloud",
        base_path=Path("/root/.cache/kagglehub/datasets/sorour/95cloud-cloud-segmentation-on-satellite-images/versions/3"),
        schedule_file="./data/95cloud/train_patches.csv",
        bands=['blue', 'green', 'red', 'nir'],
        mask_suffix="gt",
        file_extension=".TIF",
        weight=2.0,
        train_dir="95-cloud_training_only_additional_to38-cloud"
    )
}


def get_dataset_registry(config: Optional[Config] = None) -> Dict[str, DatasetConfig]:
    """
    Get dataset registry, either from config or fallback.
    
    Args:
        config: Optional Config object
    
    Returns:
        Dictionary of dataset configurations
    """
    if config is not None and config.datasets:
        return config.datasets
    return DEFAULT_DATASET_REGISTRY


class CloudSegmentationDataset(Dataset):
    
    def __init__(
        self,
        dataset_name: str,
        schedule_path: Optional[str] = None,
        transform=None,
        normalize: bool = True,
        mask_threshold: Optional[int] = None,
        target_channels: int = 4,
        augment: bool = False,
        skip_invalid: bool = True,
        sample_size: Optional[int] = None,
        config: Optional[Config] = None
    ):
        self.dataset_name = dataset_name
        self.transform = transform
        self.normalize = normalize
        self.target_channels = target_channels
        self.augment = augment
        self.skip_invalid = skip_invalid
        self.sample_size = sample_size
        self._config = config
        
        registry = get_dataset_registry(config)
        
        if dataset_name not in registry:
            raise ValueError(f"未知的数据集: {dataset_name}")
        
        self.config = registry[dataset_name]
        self.schedule_path = schedule_path or self.config.schedule_file
        
        if Path(self.schedule_path).exists():
            self.schedule_df = pd.read_csv(self.schedule_path)
        else:
            self.schedule_df = self._generate_schedule()
        
        # 过滤无效样本
        if self.skip_invalid:
            self.schedule_df = self._filter_valid_samples()
        
        # 采样（用于快速验证）
        if self.sample_size is not None and self.sample_size < len(self.schedule_df):
            self.schedule_df = self.schedule_df.sample(n=self.sample_size, random_state=42)
            print(f"快速采样: {self.config.name} -> {len(self.schedule_df)} 样本")
        
        self.mask_threshold = mask_threshold if mask_threshold is not None else self.config.mask_threshold
        
        if not self.config.base_path.exists():
            raise FileNotFoundError(f"数据集路径不存在: {self.config.base_path}")
        
        self._first_valid_sample = None
    
    def _generate_schedule(self):
        samples = []
        for ext in ['*.TIF', '*.tif', '*.png']:
            for gt_file in self.config.base_path.rglob(f"*{self.config.mask_suffix}*{ext}"):
                name = gt_file.stem.replace(f"{self.config.mask_suffix}_", "")
                samples.append({'name': name})
        return pd.DataFrame(samples)
    
    def _filter_valid_samples(self):
        valid_names = []
        
        for idx, row in self.schedule_df.iterrows():
            name = row['name']
            
            all_bands_exist = True
            for band in self.config.bands:
                if not self._check_band_exists(band, name):
                    all_bands_exist = False
                    break
            
            if all_bands_exist and not self._check_mask_exists(name):
                all_bands_exist = False
            
            if all_bands_exist:
                valid_names.append(name)
        
        print(f"过滤 {self.config.name}: 原始 {len(self.schedule_df)} -> 有效 {len(valid_names)}")
        return pd.DataFrame({'name': valid_names})
    
    def _check_band_exists(self, band: str, name: str) -> bool:
        base_path = self.config.base_path
        
        if self.config.train_dir:
            train_path = base_path / self.config.train_dir
        else:
            train_path = base_path
        
        # 检查是否有 band_files 配置（用于新格式）
        if hasattr(self.config, 'band_files'):
            band_idx = self.config.bands.index(band) if band in self.config.bands else None
            if band_idx is not None and band_idx < len(self.config.band_files):
                band_file = self.config.band_files[band_idx]
                possible_paths = [
                    train_path / "train_features" / name / band_file,
                ]
                return any(path.exists() for path in possible_paths)
        
        # 原有格式的备用
        possible_paths = [
            train_path / f"train_{band}" / f"{band}_{name}{self.config.file_extension}",
            train_path / f"train_{band}_additional_to38cloud" / f"{band}_{name}{self.config.file_extension}",
            base_path / f"train_{band}" / f"{band}_{name}{self.config.file_extension}",
        ]
        
        return any(path.exists() for path in possible_paths)
    
    def _check_mask_exists(self, name: str) -> bool:
        base_path = self.config.base_path
        
        if self.config.train_dir:
            train_path = base_path / self.config.train_dir
        else:
            train_path = base_path
        
        # 检查是否有 mask_file 配置（用于新格式）
        if hasattr(self.config, 'mask_file'):
            mask_file = self.config.mask_file.format(name=name)
            possible_paths = [
                train_path / "train_labels" / mask_file,
            ]
            if any(path.exists() for path in possible_paths):
                return True
        
        # 原有格式的备用
        possible_paths = [
            train_path / f"train_{self.config.mask_suffix}" / f"{self.config.mask_suffix}_{name}{self.config.file_extension}",
            train_path / f"train_{self.config.mask_suffix}_additional_to38cloud" / f"{self.config.mask_suffix}_{name}{self.config.file_extension}",
            base_path / f"train_{self.config.mask_suffix}" / f"{self.config.mask_suffix}_{name}{self.config.file_extension}",
        ]
        
        return any(path.exists() for path in possible_paths)
    
    def __len__(self) -> int:
        return len(self.schedule_df)
    
    def __getitem__(self, idx: int) -> Tuple[torch.Tensor, torch.Tensor]:
        name = self.schedule_df.iloc[idx]['name']
        
        try:
            image_bands = []
            for band in self.config.bands:
                band_path = self._find_band_path(band, name)
                band_data = np.array(Image.open(band_path))
                image_bands.append(band_data)
            
            image = np.stack(image_bands, axis=-1)
            image = self._pad_channels(image)
            
            mask_path = self._find_mask_path(name)
            mask = np.array(Image.open(mask_path))
            mask = (mask > self.mask_threshold).astype(np.float32)
            
            if self.augment:
                image, mask = self._apply_augmentation(image, mask)
            
            if self.normalize:
                image = self._normalize(image)
            
            image_tensor = torch.from_numpy(image).permute(2, 0, 1).float()
            mask_tensor = torch.from_numpy(mask).unsqueeze(0).float()
            
            if self.transform:
                image_tensor = self.transform(image_tensor)
                mask_tensor = self.transform(mask_tensor)
            
            return image_tensor, mask_tensor
        
        except Exception as e:
            if self.skip_invalid:
                return self._get_first_valid_sample()
            else:
                raise
    
    def _get_first_valid_sample(self):
        if self._first_valid_sample is not None:
            return self._first_valid_sample
        
        for idx in range(len(self)):
            try:
                name = self.schedule_df.iloc[idx]['name']
                
                image_bands = []
                for band in self.config.bands:
                    band_path = self._find_band_path(band, name)
                    band_data = np.array(Image.open(band_path))
                    image_bands.append(band_data)
                
                image = np.stack(image_bands, axis=-1)
                image = self._pad_channels(image)
                
                mask_path = self._find_mask_path(name)
                mask = np.array(Image.open(mask_path))
                mask = (mask > self.mask_threshold).astype(np.float32)
                
                if self.normalize:
                    image = self._normalize(image)
                
                image_tensor = torch.from_numpy(image).permute(2, 0, 1).float()
                mask_tensor = torch.from_numpy(mask).unsqueeze(0).float()
                
                self._first_valid_sample = (image_tensor, mask_tensor)
                return self._first_valid_sample
            
            except:
                continue
        
        raise RuntimeError("数据集中没有有效样本")
    
    def _pad_channels(self, image: np.ndarray) -> np.ndarray:
        current_channels = image.shape[-1]
        
        if current_channels == self.target_channels:
            return image
        
        if current_channels < self.target_channels:
            if self.config.bands == ['red', 'green', 'blue'] and self.target_channels == 4:
                nir_channel = image.mean(axis=-1, keepdims=True)
                image = np.concatenate([image, nir_channel], axis=-1)
            else:
                for _ in range(self.target_channels - current_channels):
                    last_channel = image[..., -1:]
                    image = np.concatenate([image, last_channel], axis=-1)
        
        return image
    
    def _normalize(self, image: np.ndarray) -> np.ndarray:
        if image.dtype == np.uint16:
            return image.astype(np.float32) / 65535.0
        elif image.dtype == np.uint8:
            return image.astype(np.float32) / 255.0
        return image.astype(np.float32)
    
    def _apply_augmentation(self, image: np.ndarray, mask: np.ndarray) -> Tuple[np.ndarray, np.ndarray]:
        if random.random() > 0.5:
            image = np.fliplr(image)
            mask = np.fliplr(mask)
        
        if random.random() > 0.5:
            image = np.flipud(image)
            mask = np.flipud(mask)
        
        if random.random() > 0.5:
            k = random.randint(1, 3)
            image = np.rot90(image, k)
            mask = np.rot90(mask, k)
        
        return image, mask
    
    def _find_band_path(self, band: str, name: str) -> Path:
        base_path = self.config.base_path
        
        if self.config.train_dir:
            train_path = base_path / self.config.train_dir
        else:
            train_path = base_path
        
        # 检查是否有 band_files 配置（用于新格式）
        if hasattr(self.config, 'band_files'):
            band_idx = self.config.bands.index(band) if band in self.config.bands else None
            if band_idx is not None and band_idx < len(self.config.band_files):
                band_file = self.config.band_files[band_idx]
                possible_path = train_path / "train_features" / name / band_file
                if possible_path.exists():
                    return possible_path
        
        # 原有格式的备用
        possible_paths = [
            train_path / f"train_{band}" / f"{band}_{name}{self.config.file_extension}",
            train_path / f"train_{band}_additional_to38cloud" / f"{band}_{name}{self.config.file_extension}",
            train_path / f"train_{band}" / f"train_{band}_{name}{self.config.file_extension}",
            base_path / f"train_{band}" / f"{band}_{name}{self.config.file_extension}",
            base_path / f"{band}" / f"{band}_{name}{self.config.file_extension}",
        ]
        
        for path in possible_paths:
            if path.exists():
                return path
        
        for candidate in base_path.rglob(f"*{band}*{name}{self.config.file_extension}"):
            return candidate
        
        raise FileNotFoundError(f"找不到波段文件: {band}_{name}{self.config.file_extension}")
    
    def _find_mask_path(self, name: str) -> Path:
        base_path = self.config.base_path
        
        if self.config.train_dir:
            train_path = base_path / self.config.train_dir
        else:
            train_path = base_path
        
        # 检查是否有 mask_file 配置（用于新格式）
        if hasattr(self.config, 'mask_file'):
            mask_file = self.config.mask_file.format(name=name)
            possible_path = train_path / "train_labels" / mask_file
            if possible_path.exists():
                return possible_path
        
        # 原有格式的备用
        possible_paths = [
            train_path / f"train_{self.config.mask_suffix}" / f"{self.config.mask_suffix}_{name}{self.config.file_extension}",
            train_path / f"train_{self.config.mask_suffix}_additional_to38cloud" / f"{self.config.mask_suffix}_{name}{self.config.file_extension}",
            train_path / f"train_{self.config.mask_suffix}" / f"train_{self.config.mask_suffix}_{name}{self.config.file_extension}",
            base_path / f"train_{self.config.mask_suffix}" / f"{self.config.mask_suffix}_{name}{self.config.file_extension}",
            base_path / f"{self.config.mask_suffix}" / f"{self.config.mask_suffix}_{name}{self.config.file_extension}",
        ]
        
        for path in possible_paths:
            if path.exists():
                return path
        
        for candidate in base_path.rglob(f"*{self.config.mask_suffix}*{name}{self.config.file_extension}"):
            return candidate
        
        raise FileNotFoundError(f"找不到掩码文件: {self.config.mask_suffix}_{name}{self.config.file_extension}")


class RoundRobinSampler:
    
    def __init__(self, dataset_sizes: List[int]):
        self.dataset_sizes = dataset_sizes
        self.num_datasets = len(dataset_sizes)
        self.max_size = max(dataset_sizes)
        self.current_indices = [0] * self.num_datasets
        self.dataset_order = list(range(self.num_datasets))
    
    def __iter__(self):
        for _ in range(sum(self.dataset_sizes)):
            for ds_idx in self.dataset_order:
                if self.current_indices[ds_idx] < self.dataset_sizes[ds_idx]:
                    global_idx = sum(self.dataset_sizes[:ds_idx]) + self.current_indices[ds_idx]
                    yield global_idx
                    self.current_indices[ds_idx] += 1
    
    def __len__(self):
        return sum(self.dataset_sizes)


class MultiDatasetManager:
    
    def __init__(self, dataset_names: List[str], config: Optional[Config] = None, **dataset_kwargs):
        self.dataset_names = dataset_names
        self.dataset_kwargs = dataset_kwargs
        self._config = config
        self.datasets = []
        
        registry = get_dataset_registry(config)
        
        for name in dataset_names:
            if name in registry and registry[name].base_path.exists():
                self.datasets.append(CloudSegmentationDataset(name, config=config, **dataset_kwargs))
        
        self.sizes = [len(ds) for ds in self.datasets]
        self.total_size = sum(self.sizes)
    
    def create_dataloader(
        self,
        strategy: Union[str, MergeStrategy] = "concat",
        batch_size: int = 8,
        shuffle: bool = True,
        num_workers: int = 0,
        sample_size: int = 100,  # 快速验证时每个数据集采样数量
        **dataloader_kwargs
    ) -> DataLoader:
        """根据策略创建 DataLoader"""
        
        strategy = MergeStrategy(strategy.lower()) if isinstance(strategy, str) else strategy
        
        if strategy == MergeStrategy.CONCAT:
            return self._create_concat_dataloader(batch_size, shuffle, num_workers, **dataloader_kwargs)
        
        elif strategy == MergeStrategy.WEIGHTED:
            return self._create_weighted_dataloader(batch_size, num_workers, **dataloader_kwargs)
        
        elif strategy == MergeStrategy.ROUND_ROBIN:
            return self._create_round_robin_dataloader(batch_size, num_workers, **dataloader_kwargs)
        
        elif strategy == MergeStrategy.BALANCED:
            return self._create_balanced_dataloader(batch_size, num_workers, **dataloader_kwargs)
        
        elif strategy == MergeStrategy.SAMPLE:
            return self._create_sample_dataloader(batch_size, shuffle, num_workers, sample_size, **dataloader_kwargs)
        
        else:
            raise ValueError(f"未知的合并策略: {strategy}")
    
    def _create_concat_dataloader(self, batch_size, shuffle, num_workers, **kwargs):
        concat_dataset = ConcatDataset(self.datasets)
        return DataLoader(concat_dataset, batch_size=batch_size, shuffle=shuffle, num_workers=num_workers, **kwargs)
    
    def _create_weighted_dataloader(self, batch_size, num_workers, **kwargs):
        registry = get_dataset_registry(self._config)
        weights = []
        for i, ds in enumerate(self.datasets):
            ds_name = self.dataset_names[i]
            ds_weight = registry[ds_name].weight
            weights.extend([ds_weight] * len(ds))
        
        sampler = WeightedRandomSampler(weights, num_samples=self.total_size, replacement=True)
        concat_dataset = ConcatDataset(self.datasets)
        return DataLoader(concat_dataset, batch_size=batch_size, sampler=sampler, num_workers=num_workers, **kwargs)
    
    def _create_round_robin_dataloader(self, batch_size, num_workers, **kwargs):
        sampler = RoundRobinSampler(self.sizes)
        concat_dataset = ConcatDataset(self.datasets)
        return DataLoader(concat_dataset, batch_size=batch_size, sampler=sampler, num_workers=num_workers, **kwargs)
    
    def _create_balanced_dataloader(self, batch_size, num_workers, **kwargs):
        min_size = min(self.sizes)
        
        balanced_indices = []
        for ds_idx, ds in enumerate(self.datasets):
            indices = random.sample(range(len(ds)), min_size)
            indices = [i + sum(self.sizes[:ds_idx]) for i in indices]
            balanced_indices.extend(indices)
        
        random.shuffle(balanced_indices)
        
        class BalancedSampler(torch.utils.data.Sampler):
            def __init__(self, indices):
                self.indices = indices
            
            def __iter__(self):
                return iter(self.indices)
            
            def __len__(self):
                return len(self.indices)
        
        sampler = BalancedSampler(balanced_indices)
        concat_dataset = ConcatDataset(self.datasets)
        return DataLoader(concat_dataset, batch_size=batch_size, sampler=sampler, num_workers=num_workers, **kwargs)
    
    def _create_sample_dataloader(self, batch_size, shuffle, num_workers, sample_size, **kwargs):
        """
        快速验证采样策略
        
        从每个数据集随机采样指定数量的样本，用于快速验证模型性能。
        特点：
        - 每个数据集采样数量相等（默认100个）
        - 速度快，适合快速验证
        - 可用于训练前的快速测试
        """
        sample_datasets = []
        
        for ds in self.datasets:
            # 从每个数据集采样指定数量的样本
            if len(ds) > sample_size:
                indices = np.random.choice(len(ds), sample_size, replace=False)
                sample_ds = Subset(ds, indices)
            else:
                sample_ds = ds
            
            sample_datasets.append(sample_ds)
        
        concat_dataset = ConcatDataset(sample_datasets)
        print(f"快速采样策略: {len(sample_datasets)} 个数据集，每个采样 {sample_size} 个样本")
        print(f"总样本数: {len(concat_dataset)}")
        
        return DataLoader(concat_dataset, batch_size=batch_size, shuffle=shuffle, num_workers=num_workers, **kwargs)


def get_combined_dataloader(
    dataset_names: List[str],
    strategy: str = "concat",
    batch_size: int = 8,
    shuffle: bool = True,
    num_workers: int = 0,
    sample_size: int = 100,
    config: Optional[Config] = None,
    **dataset_kwargs
) -> DataLoader:
    """便捷函数：创建合并数据集的 DataLoader"""
    manager = MultiDatasetManager(dataset_names, config=config, **dataset_kwargs)
    return manager.create_dataloader(
        strategy=strategy, 
        batch_size=batch_size, 
        shuffle=shuffle, 
        num_workers=num_workers,
        sample_size=sample_size
    )


def list_available_datasets_with_stats(config: Optional[Config] = None) -> List[Dict[str, Any]]:
    result = []
    registry = get_dataset_registry(config)
    
    for key, ds_config in registry.items():
        available = ds_config.base_path.exists()
        sample_count = 0
        
        if available:
            schedule_path = Path(ds_config.schedule_file)
            if schedule_path.exists():
                df = pd.read_csv(schedule_path)
                sample_count = len(df)
            else:
                tif_count = len(list(ds_config.base_path.rglob("*.TIF")))
                sample_count = tif_count // (len(ds_config.bands) + 1)
        
        result.append({
            "key": key,
            "name": ds_config.name,
            "bands": ds_config.bands,
            "num_bands": len(ds_config.bands),
            "file_extension": ds_config.file_extension,
            "weight": ds_config.weight,
            "available": available,
            "sample_count": sample_count,
            "base_path": str(ds_config.base_path)
        })
    
    return result


def print_dataset_summary(config: Optional[Config] = None):
    datasets = list_available_datasets_with_stats(config)
    
    print("=" * 80)
    print("数据集摘要")
    print("=" * 80)
    
    print(f"\n{'数据集':<20} {'波段':<15} {'文件格式':<10} {'样本数':<8} {'权重':<6} {'状态'}")
    print("-" * 80)
    
    for ds in datasets:
        status = "✓" if ds['available'] else "✗"
        bands_str = ", ".join(ds['bands'])[:15]
        print(f"{ds['name']:<20} {bands_str:<15} {ds['file_extension']:<10} {ds['sample_count']:<8} {ds['weight']:<6} {status}")
    
    print("\n" + "=" * 80)


def print_strategy_summary():
    """打印所有采样策略的说明"""
    strategies = [
        ("concat", "简单拼接", "将所有数据集简单拼接在一起，保持原始比例", "训练"),
        ("weighted", "加权采样", "根据权重配置随机采样，权重高的数据集采样概率更高", "训练"),
        ("round_robin", "轮询采样", "按顺序从每个数据集取样本，确保均匀采样", "训练"),
        ("balanced", "平衡采样", "从每个数据集取相同数量的样本", "训练/验证"),
        ("sample", "快速验证采样", "从每个数据集采样少量样本（默认100个），用于快速验证", "快速验证")
    ]
    
    print("=" * 80)
    print("采样策略说明")
    print("=" * 80)
    
    print(f"\n{'策略':<15} {'名称':<12} {'说明'}")
    print("-" * 80)
    
    for strategy, name, desc, usage in strategies:
        print(f"{strategy:<15} {name:<12} {desc}")
        print(f"{'':<27} 适用场景: {usage}")
    
    print("\n" + "=" * 80)


if __name__ == "__main__":
    # Try to load config
    try:
        from utils import load_config
        config = load_config()
        print_dataset_summary(config)
    except ImportError:
        config = None
        print_dataset_summary()
    
    print("\n\n" + "=" * 80)
    print("示例: 创建不同策略的 DataLoader")
    print("=" * 80)
    
    print("\n1. Concat 策略 (简单拼接):")
    loader = get_combined_dataloader(
        dataset_names=["38cloud", "95cloud"],
        strategy="concat",
        batch_size=4,
        config=config
    )
    print(f"   总样本数: {len(loader.dataset)}")
    
    print("\n2. Weighted 策略 (加权采样):")
    loader = get_combined_dataloader(
        dataset_names=["38cloud", "95cloud"],
        strategy="weighted",
        batch_size=4,
        config=config
    )
    print(f"   权重配置: 38cloud=1.0, 95cloud=2.0")
    
    print("\n3. Round Robin 策略 (轮询采样):")
    loader = get_combined_dataloader(
        dataset_names=["38cloud", "95cloud"],
        strategy="round_robin",
        batch_size=4,
        config=config
    )
    print(f"   轮询顺序: 38cloud -> 95cloud -> 38cloud -> ...")
    
    print("\n4. Balanced 策略 (平衡采样):")
    loader = get_combined_dataloader(
        dataset_names=["38cloud", "95cloud"],
        strategy="balanced",
        batch_size=4,
        config=config
    )
    print(f"   每个数据集取相同数量样本")
    
    print("\n5. Sample 策略 (快速验证采样):")
    loader = get_combined_dataloader(
        dataset_names=["38cloud", "95cloud"],
        strategy="sample",
        batch_size=4,
        sample_size=50,
        config=config
    )
    print(f"   每个数据集采样 50 个样本，总样本数: {len(loader.dataset)}")
    
    print("\n" + "=" * 80)
    print_strategy_summary()
