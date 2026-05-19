"""
完整的问题分析脚本
"""
import torch
import numpy as np
from pathlib import Path
from PIL import Image
from collections import Counter


def analyze_all_samples(base_path):
    """分析所有样本"""
    gt_dir = base_path / "38-Cloud_training/train_gt"
    
    all_samples = []
    
    print("=== 分析所有有效样本 ===")
    
    for gt_file in sorted(gt_dir.glob("*.TIF"))[:100]:
        gt = np.array(Image.open(gt_file))
        
        if gt.max() > 0:
            name = gt_file.stem.replace("gt_", "")
            cloud_ratio = (gt > 0).mean()
            
            all_samples.append({
                'name': name,
                'cloud_ratio': cloud_ratio
            })
    
    print(f"共找到 {len(all_samples)} 个有效样本\n")
    
    # 分析云覆盖率分布
    print("=== 云覆盖率统计 ===")
    ratios = [s['cloud_ratio'] for s in all_samples]
    print(f"  最小: {min(ratios):.4f}")
    print(f"  最大: {max(ratios):.4f}")
    print(f"  平均: {np.mean(ratios):.4f}")
    
    # 找出全云样本
    all_cloud = [s for s in all_samples if s['cloud_ratio'] > 0.99]
    no_cloud = [s for s in all_samples if s['cloud_ratio'] < 0.01]
    
    print(f"\n  全云样本 (云覆盖率>99%): {len(all_cloud)} 个")
    for s in all_cloud[:3]:
        print(f"    - {s['name'][:50]}")
    print(f"\n  无云样本 (云覆盖率<1%): {len(no_cloud)} 个")
    
    # 检查训练和验证
    train_names = [s['name'] for s in all_samples[:30]]
    val_names = [s['name'] for s in all_samples[30:40]]
    
    print(f"\n=== 训练验证样本分析 ===")
    print(f"训练集样本数: {len(train_names)}")
    print(f"验证集样本数: {len(val_names)}")
    
    train_cloud = [s['cloud_ratio'] for s in all_samples[:30]]
    val_cloud = [s['cloud_ratio'] for s in all_samples[30:40]]
    
    print(f"\n训练集云覆盖率:")
    print(f"  最小值: {min(train_cloud):.4f}")
    print(f"  最大值: {max(train_cloud):.4f}")
    print(f"  平均: {np.mean(train_cloud):.4f}")
    print(f"  全云样本: {sum(1 for x in train_cloud if x>0.99)}")
    
    print(f"\n验证集云覆盖率:")
    print(f"  最小值: {min(val_cloud):.4f}")
    print(f"  最大值: {max(val_cloud):.4f}")
    print(f"  平均: {np.mean(val_cloud):.4f}")
    print(f"  全云样本: {sum(1 for x in val_cloud if x>0.99)}")
    
    # 关键问题：检查全云样本
    print(f"\n⚠️ 关键发现：")
    if any(1 for x in val_cloud if x>0.99):
        print("  验证集中包含全云样本！模型只要全预测1就能得到完美指标！")
        
    # 检查我们的验证脚本
    print(f"\n=== 验证脚本问题 ===")
    print("1. 验证脚本的样本来自: all_samples[:5]")
    print("2. 训练脚本的训练样本来自: all_samples[:30]")
    print("3. **问题！验证脚本用的是训练集的前5个样本！**")
    print("   这是在验证训练数据，而不是独立的验证集！")
    
    # 模拟一下：如果预测全1，会得到什么结果
    print(f"\n=== 模拟测试：如果全预测1 ===")
    test_cloud = val_cloud[1]  # 第二个全云样本
    if test_cloud > 0.99:
        print(f"第二个验证样本云覆盖率: {test_cloud*100:.2f}%")
        print("如果模型全预测1:")
        print("  Dice = 2*(1*1)/(1+1) = 1.0")
        print("  IoU = 1/1 = 1.0")
        print("这就是为什么会有完美得分！")


if __name__ == "__main__":
    base_path = Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4")
    analyze_all_samples(base_path)
