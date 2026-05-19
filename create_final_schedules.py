
import pandas as pd
from pathlib import Path
import shutil


def process_38cloud_dataset():
    base_path = Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4")
    output_dir = Path("/workspace/data/38cloud")
    output_dir.mkdir(parents=True, exist_ok=True)
    
    print(f"\n=== 处理 38cloud 数据集 ===")
    
    # 查找训练和测试 CSV 文件
    training_csv = base_path / "38-Cloud_training" / "training_patches_38-Cloud.csv"
    test_csv = base_path / "38-Cloud_test" / "test_patches_38-Cloud.csv"
    
    if training_csv.exists():
        df_train = pd.read_csv(training_csv)
        df_train.to_csv(output_dir / "train_patches.csv", index=False)
        print(f"训练样本: {len(df_train)}")
        print(f"训练 CSV 列: {df_train.columns.tolist()}")
    
    if test_csv.exists():
        df_test = pd.read_csv(test_csv)
        df_test.to_csv(output_dir / "test_patches.csv", index=False)
        print(f"测试样本: {len(df_test)}")
        print(f"测试 CSV 列: {df_test.columns.tolist()}")
    
    return base_path


def process_95cloud_dataset():
    base_path = Path("/root/.cache/kagglehub/datasets/sorour/95cloud-cloud-segmentation-on-satellite-images/versions/3")
    output_dir = Path("/workspace/data/95cloud")
    output_dir.mkdir(parents=True, exist_ok=True)
    
    print(f"\n=== 处理 95cloud 数据集 ===")
    
    # 查找训练 CSV 文件
    training_csv = base_path / "95-cloud_training_only_additional_to38-cloud" / "training_patches_95-cloud.csv"
    combined_csv = base_path / "95-cloud_training_only_additional_to38-cloud" / "training_patches_95-cloud_additional_to_38-cloud.csv"
    
    if training_csv.exists():
        df_train = pd.read_csv(training_csv)
        df_train.to_csv(output_dir / "train_patches.csv", index=False)
        print(f"训练样本: {len(df_train)}")
        print(f"训练 CSV 列: {df_train.columns.tolist()}")
    
    if combined_csv.exists():
        df_combined = pd.read_csv(combined_csv)
        df_combined.to_csv(output_dir / "combined_train_patches.csv", index=False)
        print(f"合并样本: {len(df_combined)}")
    
    return base_path


def create_master_schedule():
    print(f"\n=== 创建主调度文件 ===")
    schedules = []
    
    # 38cloud
    path_38 = Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4")
    train_csv_38 = path_38 / "38-Cloud_training" / "training_patches_38-Cloud.csv"
    if train_csv_38.exists():
        df_38 = pd.read_csv(train_csv_38)
        df_38['dataset'] = '38cloud'
        df_38['split'] = 'train'
        schedules.append(df_38)
    
    # 95cloud
    path_95 = Path("/root/.cache/kagglehub/datasets/sorour/95cloud-cloud-segmentation-on-satellite-images/versions/3")
    train_csv_95 = path_95 / "95-cloud_training_only_additional_to38-cloud" / "training_patches_95-cloud.csv"
    if train_csv_95.exists():
        df_95 = pd.read_csv(train_csv_95)
        df_95['dataset'] = '95cloud'
        df_95['split'] = 'train'
        schedules.append(df_95)
    
    if schedules:
        master_df = pd.concat(schedules, ignore_index=True)
        master_df.to_csv("/workspace/data/master_schedule.csv", index=False)
        print(f"主调度文件创建完成: {len(master_df)} 条记录")
        print(f"主调度文件保存到: /workspace/data/master_schedule.csv")


if __name__ == "__main__":
    process_38cloud_dataset()
    process_95cloud_dataset()
    create_master_schedule()
    
    print("\n数据处理完成！")
    print("调度文件已保存到 /workspace/data/ 目录")
