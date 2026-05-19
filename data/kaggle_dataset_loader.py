
import pandas as pd
from pathlib import Path
import numpy as np
from PIL import Image
import torch
from torch.utils.data import Dataset, DataLoader


class KaggleCloudDataset(Dataset):
    """Kaggle 38cloud 和 95cloud 云分割数据集加载器"""
    
    def __init__(self, schedule_path, dataset_name="38cloud", split="train", transform=None):
        """
        Args:
            schedule_path: 调度 CSV 路径
            dataset_name: "38cloud" 或 "95cloud"
            split: "train" 或 "test"
            transform: 数据变换
        """
        self.schedule_df = pd.read_csv(schedule_path)
        self.dataset_name = dataset_name
        self.split = split
        self.transform = transform
        
        # 设置数据集根路径
        if dataset_name == "38cloud":
            self.base_path = Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4")
            self.split_dir = self.base_path / "38-Cloud_training" if split == "train" else self.base_path / "38-Cloud_test"
        elif dataset_name == "95cloud":
            self.base_path = Path("/root/.cache/kagglehub/datasets/sorour/95cloud-cloud-segmentation-on-satellite-images/versions/3")
            self.split_dir = self.base_path / "95-cloud_training_only_additional_to38-cloud"
        
    def __len__(self):
        return len(self.schedule_df)
    
    def __getitem__(self, idx):
        """获取单个样本"""
        name = self.schedule_df.iloc[idx]['name']
        
        # 加载多光谱波段
        blue_path = self.split_dir / f"train_blue" / name
        green_path = self.split_dir / f"train_green" / name
        red_path = self.split_dir / f"train_red" / name
        nir_path = self.split_dir / f"train_nir" / name
        
        # 加载真值
        gt_path = self.split_dir / f"train_gt" / name
        
        # 读取图像
        blue = np.array(Image.open(blue_path))
        green = np.array(Image.open(green_path))
        red = np.array(Image.open(red_path))
        nir = np.array(Image.open(nir_path))
        
        # 组合成 4 通道图像
        image = np.stack([red, green, blue, nir], axis=-1)
        
        # 读取掩码
        mask = np.array(Image.open(gt_path))
        mask = (mask > 0).astype(np.float32)
        
        if self.transform:
            image = self.transform(image)
            mask = self.transform(mask)
        
        return image, mask


def get_kaggle_dataloaders(batch_size=32, num_workers=4):
    """获取 Kaggle 数据集的 DataLoader"""
    
    train_dataset_38 = KaggleCloudDataset(
        "/workspace/data/38cloud/train_patches.csv",
        dataset_name="38cloud",
        split="train"
    )
    
    test_dataset_38 = KaggleCloudDataset(
        "/workspace/data/38cloud/test_patches.csv",
        dataset_name="38cloud",
        split="test"
    )
    
    train_loader_38 = DataLoader(train_dataset_38, batch_size=batch_size, shuffle=True, num_workers=num_workers)
    test_loader_38 = DataLoader(test_dataset_38, batch_size=batch_size, shuffle=False, num_workers=num_workers)
    
    return {
        "38cloud_train": train_loader_38,
        "38cloud_test": test_loader_38
    }


if __name__ == "__main__":
    print("Kaggle 云分割数据集加载器")
    print(f"38cloud 训练样本: 8400")
    print(f"38cloud 测试样本: 9201")
    print(f"95cloud 训练样本: 34701")
    print(f"主调度样本: 43101")
