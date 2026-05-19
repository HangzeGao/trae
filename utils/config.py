"""
Configuration management for cloud segmentation project.
"""
import yaml
from pathlib import Path
from typing import Dict, Any, Optional
from dataclasses import dataclass, field
from enum import Enum


class MergeStrategy(Enum):
    """Dataset merge strategies"""
    CONCAT = "concat"
    WEIGHTED = "weighted"
    ROUND_ROBIN = "round_robin"
    BALANCED = "balanced"
    SAMPLE = "sample"


@dataclass
class DatasetConfig:
    """Dataset configuration"""
    name: str
    base_path: Path
    schedule_file: str
    bands: list = field(default_factory=lambda: ['blue', 'green', 'red', 'nir'])
    mask_suffix: str = "gt"
    mask_threshold: int = 127
    file_extension: str = ".TIF"
    weight: float = 1.0
    train_dir: Optional[str] = None
    
    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> 'DatasetConfig':
        """Create DatasetConfig from dictionary"""
        data = data.copy()
        if 'base_path' in data:
            data['base_path'] = Path(data['base_path'])
        return cls(**data)


@dataclass
class Config:
    """Main configuration class"""
    model: Dict[str, Any] = field(default_factory=dict)
    training: Dict[str, Any] = field(default_factory=dict)
    data: Dict[str, Any] = field(default_factory=dict)
    datasets: Dict[str, DatasetConfig] = field(default_factory=dict)
    sampling: Dict[str, Any] = field(default_factory=dict)
    logging: Dict[str, Any] = field(default_factory=dict)
    
    @classmethod
    def from_yaml(cls, config_path: Path) -> 'Config':
        """Load config from YAML file"""
        with open(config_path, 'r', encoding='utf-8') as f:
            data = yaml.safe_load(f)
        
        config = cls()
        
        if 'model' in data:
            config.model = data['model']
        
        if 'training' in data:
            config.training = data['training']
        
        if 'data' in data:
            config.data = data['data']
        
        if 'datasets' in data:
            config.datasets = {
                name: DatasetConfig.from_dict(ds_data)
                for name, ds_data in data['datasets'].items()
            }
        
        if 'sampling' in data:
            config.sampling = data['sampling']
        
        if 'logging' in data:
            config.logging = data['logging']
        
        return config
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert config to dictionary"""
        return {
            'model': self.model,
            'training': self.training,
            'data': self.data,
            'datasets': {
                name: {
                    k: str(v) if isinstance(v, Path) else v
                    for k, v in ds.__dict__.items()
                }
                for name, ds in self.datasets.items()
            },
            'sampling': self.sampling,
            'logging': self.logging
        }
    
    def save(self, config_path: Path):
        """Save config to YAML file"""
        with open(config_path, 'w', encoding='utf-8') as f:
            yaml.dump(self.to_dict(), f, default_flow_style=False, allow_unicode=True)
    
    def get_dataset_config(self, dataset_name: str) -> Optional[DatasetConfig]:
        """Get dataset config by name"""
        return self.datasets.get(dataset_name)
    
    def list_available_datasets(self) -> list:
        """List all available datasets with status"""
        result = []
        for name, ds_config in self.datasets.items():
            available = ds_config.base_path.exists()
            result.append({
                'name': name,
                'display_name': ds_config.name,
                'available': available,
                'base_path': str(ds_config.base_path)
            })
        return result


def get_config_path() -> Path:
    """Get default config file path"""
    return Path(__file__).parent.parent / 'config' / 'config.yaml'


def load_config(config_path: Optional[Path] = None) -> Config:
    """
    Load configuration from YAML file.
    
    Args:
        config_path: Path to config file, uses default if None
    
    Returns:
        Config object
    """
    if config_path is None:
        config_path = get_config_path()
    
    return Config.from_yaml(config_path)


if __name__ == '__main__':
    # Test config loading
    config = load_config()
    print("Config loaded successfully:")
    print(f"  Model: {config.model.get('architecture')}")
    print(f"  Datasets: {list(config.datasets.keys())}")
    print(f"  Default strategy: {config.sampling.get('default_strategy')}")
