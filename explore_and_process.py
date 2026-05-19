
import kagglehub
from pathlib import Path
import pandas as pd
import shutil


def explore_38cloud_dataset():
    path = "/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4"
    base_path = Path(path)
    
    print(f"\n=== 38cloud 数据集结构 ===")
    print(f"数据集路径: {base_path}")
    
    # 列出顶层目录
    print("\n顶层目录:")
    for item in sorted(base_path.iterdir()):
        if item.is_dir():
            print(f"  - {item.name}")
    
    return base_path


def create_csv_schedule(base_path, output_path):
    # 探索常见的数据集结构
    image_dirs = []
    mask_dirs = []
    
    # 查找可能的图像和掩码文件夹
    for dir_path in base_path.rglob("*"):
        if dir_path.is_dir():
            name_lower = dir_path.name.lower()
            if any(keyword in name_lower for keyword in ['image', 'img', 'train', 'test', 'val']):
                image_dirs.append(dir_path)
            if any(keyword in name_lower for keyword in ['mask', 'label', 'gt', 'seg']):
                mask_dirs.append(dir_path)
    
    print(f"\n找到的图像目录: {[d.name for d in image_dirs]}")
    print(f"找到的掩码目录: {[d.name for d in mask_dirs]}")
    
    # 创建简单的调度文件示例
    schedule_data = []
    
    # 先尝试找到一些配对的图像和掩码
    # 这需要根据实际数据集结构调整
    data = []
    for item in base_path.rglob("*.png"):
        data.append(str(item))
    for item in base_path.rglob("*.tif"):
        data.append(str(item))
    for item in base_path.rglob("*.jpg"):
        data.append(str(item))
    
    print(f"\n找到 {len(data)} 个图像文件")
    
    # 创建 CSV 文件
    df = pd.DataFrame(data, columns=['file_path'])
    df.to_csv(output_path, index=False, encoding='utf-8')
    print(f"\n调度文件已保存到: {output_path}")
    
    return df


def retry_download_95cloud():
    print("\n=== 尝试重新下载 95cloud 数据集 ===")
    try:
        path = kagglehub.dataset_download("sorour/95cloud-cloud-segmentation-on-satellite-images")
        print(f"95cloud 数据集下载成功: {path}")
        return path
    except Exception as e:
        print(f"95cloud 下载失败: {e}")
        return None


if __name__ == "__main__":
    # 探索 38cloud
    path_38 = explore_38cloud_dataset()
    
    # 创建 CSV 调度文件
    create_csv_schedule(path_38, "/workspace/38cloud_schedule.csv")
    
    # 尝试下载 95cloud
    path_95 = retry_download_95cloud()
    if path_95:
        create_csv_schedule(Path(path_95), "/workspace/95cloud_schedule.csv")
