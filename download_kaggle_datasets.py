
import kagglehub
import os
from pathlib import Path


def download_38cloud_dataset():
    print("正在下载 38cloud 数据集...")
    path = kagglehub.dataset_download("sorour/38cloud-cloud-segmentation-in-satellite-images")
    print(f"38cloud 数据集路径: {path}")
    return path


def download_95cloud_dataset():
    print("正在下载 95cloud 数据集...")
    path = kagglehub.dataset_download("sorour/95cloud-cloud-segmentation-on-satellite-images")
    print(f"95cloud 数据集路径: {path}")
    return path


def explore_dataset_structure(path, dataset_name):
    print(f"\n=== {dataset_name} 数据集结构 ===")
    base_path = Path(path)
    for item in sorted(base_path.rglob("*")):
        if item.is_file():
            print(f"  {item.relative_to(base_path)}")
    print()


if __name__ == "__main__":
    path_38cloud = download_38cloud_dataset()
    path_95cloud = download_95cloud_dataset()
    
    explore_dataset_structure(path_38cloud, "38cloud")
    explore_dataset_structure(path_95cloud, "95cloud")
