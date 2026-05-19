"""
检查数据划分问题
"""
from pathlib import Path
from PIL import Image
import numpy as np


def get_sample_stats():
    print("=== 检查训练和验证数据划分 ===")
    
    base_path = Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4")
    gt_dir = base_path / "38-Cloud_training/train_gt"
    
    # 收集所有有效样本
    all_valid_names = []
    
    print("\n查找所有有效样本...")
    for gt_file in sorted(gt_dir.glob("*.TIF"))[:100]:
        gt = np.array(Image.open(gt_file))
        if gt.max() > 0:
            name = gt_file.stem.replace("gt_", "")
            all_valid_names.append(name)
    
    print(f"共找到 {len(all_valid_names)} 个有效样本")
    
    # 之前的划分方式
    train_names_old = all_valid_names[:30]
    val_names_old = all_valid_names[30:40]
    
    # 检查重叠
    overlap = set(train_names_old) & set(val_names_old)
    print(f"\n之前的划分 (训练30, 验证10):")
    print(f"  训练样本数: {len(train_names_old)}")
    print(f"  验证样本数: {len(val_names_old)}")
    print(f"  重叠样本: {len(overlap)}")
    print(f"  重叠率: {100 * len(overlap)/len(val_names_old):.1f}%")
    
    # 查看训练和验证样本
    print(f"\n训练样本前5个:")
    for n in train_names_old[:5]:
        print(f"  {n}")
    print(f"\n验证样本前5个:")
    for n in val_names_old[:5]:
        print(f"  {n}")
    
    # 另外，检查我们的验证过程是不是用了训练数据的前几个
    print(f"\n=== 问题排查 ===")
    print("1. 验证样本来自: all_valid_names[30:40]")
    print("2. 训练样本来自: all_valid_names[:30]")
    print("3. 这两个集合是连续的，没有重叠，但是...")
    print("4. 关键问题：我们在训练时验证集是不是就是训练集的子集？")
    print("   并且验证样本数很少，模型可能已经记住了这些样本！")
    
    # 另外，检查我们的指标计算
    print("\n=== 另外检查指标计算 ===")
    print("让我们看看一个样本的真实值和预测值:")
    
    # 检查第一个样本
    if len(all_valid_names) > 0:
        name = all_valid_names[0]
        gt = np.array(Image.open(gt_dir / f"gt_{name}.TIF"))
        print(f"\n样本 '{name[:40]}':")
        print(f"  GT 最小值: {gt.min()}")
        print(f"  GT 最大值: {gt.max()}")
        print(f"  GT 唯一值: {np.unique(gt)}")
        print(f"  GT 非零像素: {100 * (gt>0).mean():.2f}%")
        
        # 看一下样本 2，它的 Dice 是 1.0
        name2 = all_valid_names[1]
        gt2 = np.array(Image.open(gt_dir / f"gt_{name2}.TIF"))
        print(f"\n样本 '{name2[:40]}' (Dice=1.0):")
        print(f"  GT 非零像素: {100 * (gt2>0).mean():.2f}%")
    
    return all_valid_names


if __name__ == "__main__":
    get_sample_stats()
