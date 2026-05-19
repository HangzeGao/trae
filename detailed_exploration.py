
from pathlib import Path
import pandas as pd


def explore_dataset_detailed(base_path, dataset_name):
    print(f"\n{'='*60}")
    print(f"{dataset_name} 数据集详细结构")
    print(f"{'='*60}")
    print(f"路径: {base_path}")
    
    # 列出所有子目录
    print(f"\n所有子目录:")
    for item in sorted(base_path.iterdir()):
        if item.is_dir():
            print(f"  - {item.name}")
            # 查看每个子目录的内容
            sub_items = sorted(item.iterdir())
            count = 0
            for sub_item in sub_items:
                if count < 10:
                    print(f"    - {sub_item.name}")
                    count += 1
            len_sub = len(list(item.iterdir()))
            if len_sub > 10:
                print(f"    ... (共 {len_sub} 项)")
    
    # 查找所有图像和掩码文件
    print(f"\n查找图像/掩码文件:")
    extensions = ['.png', '.tif', '.tiff', '.jpg', '.jpeg', '.bmp']
    all_files = []
    for ext in extensions:
        files = list(base_path.rglob(f'*{ext}'))
        all_files.extend(files)
        if files:
            print(f"  - {ext}: {len(files)} 个文件")
    
    print(f"\n总计找到 {len(all_files)} 个图像/掩码文件")
    
    # 创建完整的CSV调度文件
    df = pd.DataFrame([str(f) for f in all_files], columns=['file_path'])
    df['filename'] = df['file_path'].apply(lambda x: Path(x).name)
    df['extension'] = df['file_path'].apply(lambda x: Path(x).suffix)
    df['parent_dir'] = df['file_path'].apply(lambda x: Path(x).parent.name)
    
    return df


if __name__ == "__main__":
    # 38cloud 数据集
    path_38 = Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4")
    df_38 = explore_dataset_detailed(path_38, "38cloud")
    df_38.to_csv("/workspace/38cloud_detailed.csv", index=False, encoding='utf-8')
    print(f"\n38cloud 详细调度文件保存到 /workspace/38cloud_detailed.csv")
    print(f"  共 {len(df_38)} 行")
    
    # 95cloud 数据集
    path_95 = Path("/root/.cache/kagglehub/datasets/sorour/95cloud-cloud-segmentation-on-satellite-images/versions/3")
    df_95 = explore_dataset_detailed(path_95, "95cloud")
    df_95.to_csv("/workspace/95cloud_detailed.csv", index=False, encoding='utf-8')
    print(f"\n95cloud 详细调度文件保存到 /workspace/95cloud_detailed.csv")
    print(f"  共 {len(df_95)} 行")
    
    print("\n数据集下载和调度文件生成完成！")
