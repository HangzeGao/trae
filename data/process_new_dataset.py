"""
处理新下载的 Kaggle Cloud Cover Detection 数据集
"""
import zipfile
from pathlib import Path
import pandas as pd
import shutil


def extract_and_explore_dataset():
    """解压并探索数据集结构"""
    base_path = Path("/root/.cache/kagglehub/datasets/hmendonca/cloud-cover-detection")
    archive_path = base_path / "3.archive"
    
    print("=" * 60)
    print("处理 Cloud Cover Detection 数据集")
    print("=" * 60)
    
    # 解压
    print("\n1. 解压数据集...")
    extract_dir = base_path / "extracted"
    extract_dir.mkdir(exist_ok=True)
    
    try:
        with zipfile.ZipFile(archive_path, 'r') as zip_ref:
            zip_ref.extractall(extract_dir)
        print(f"   ✓ 已解压到: {extract_dir}")
    except Exception as e:
        print(f"   ✗ 解压失败: {e}")
        return
    
    # 探索结构
    print("\n2. 探索数据集结构...")
    for item in sorted(extract_dir.rglob("*"))[:30]:
        if item.is_file():
            print(f"   {item.relative_to(extract_dir)}")
    
    return extract_dir


def create_schedule_for_cloud_cover():
    """为 cloud-cover 数据集创建调度文件"""
    extract_dir = Path("/root/.cache/kagglehub/datasets/hmendonca/cloud-cover-detection/extracted")
    
    print("\n3. 创建调度文件...")
    
    # 创建数据目录
    data_dir = Path("/workspace/data/cloud-cover")
    data_dir.mkdir(exist_ok=True)
    
    # 查找图像文件
    image_files = []
    
    for ext in ['*.png', '*.jpg', '*.tif', '*.TIF']:
        image_files.extend(sorted(extract_dir.rglob(ext)))
    
    print(f"   找到 {len(image_files)} 个图像文件")
    
    if image_files:
        # 创建调度文件
        schedule_data = []
        
        for img_path in image_files:
            name = img_path.stem
            schedule_data.append({'name': name, 'path': str(img_path)})
        
        schedule_df = pd.DataFrame(schedule_data)
        schedule_path = data_dir / "train_patches.csv"
        schedule_df.to_csv(schedule_path, index=False)
        
        print(f"   ✓ 调度文件已创建: {schedule_path}")
        print(f"   样本数: {len(schedule_df)}")
        
        # 显示示例
        print("\n4. 样本示例:")
        print(schedule_df.head())


if __name__ == "__main__":
    extract_dir = extract_and_explore_dataset()
    if extract_dir:
        create_schedule_for_cloud_cover()
