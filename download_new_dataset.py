"""
下载新的 Kaggle 云覆盖检测数据集
"""
import kagglehub
from pathlib import Path


def main():
    print("=" * 60)
    print("下载 Kaggle 数据集")
    print("=" * 60)
    
    # 下载数据集
    dataset_id = "hmendonca/cloud-cover-detection"
    print(f"\n正在下载数据集: {dataset_id}")
    
    path = kagglehub.dataset_download(dataset_id)
    print(f"✓ 数据集已下载到: {path}")
    
    # 查看数据集结构
    dataset_path = Path(path)
    print(f"\n数据集结构:")
    
    def print_dir_tree(root, prefix="  "):
        """打印目录树"""
        if not root.is_dir():
            return
        
        for item in sorted(root.iterdir()):
            print(f"{prefix}{item.name}")
            if item.is_dir():
                print_dir_tree(item, prefix + "  ")
    
    print_dir_tree(dataset_path)
    
    print("\n" + "=" * 60)
    print("下载完成！")
    print("=" * 60)


if __name__ == "__main__":
    main()
