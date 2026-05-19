"""
调试路径查找问题
"""
from pathlib import Path

# 测试参数
dataset_name = "38cloud"
band = "blue"
name = "patch_1_1_by_1_LC08_L1TP_002053_20160520_20170324_01_T1"
file_extension = ".TIF"
train_dir = "38-Cloud_training"

base_path = Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4")

print(f"测试样本: {name}")
print(f"数据集路径: {base_path}")
print()

# 尝试各种路径格式
possible_paths = [
    # 格式1: train_path/train_band/band_name.TIF
    base_path / train_dir / f"train_{band}" / f"{band}_{name}{file_extension}",
    
    # 格式2: train_path/train_band_additional_to38cloud/band_name.TIF
    base_path / train_dir / f"train_{band}_additional_to38cloud" / f"{band}_{name}{file_extension}",
    
    # 格式3: train_path/train_band/train_band_name.TIF
    base_path / train_dir / f"train_{band}" / f"train_{band}_{name}{file_extension}",
    
    # 格式4: base_path/train_band/band_name.TIF
    base_path / f"train_{band}" / f"{band}_{name}{file_extension}",
    
    # 格式5: base_path/band/band_name.TIF
    base_path / f"{band}" / f"{band}_{name}{file_extension}",
]

print("尝试的路径:")
for i, path in enumerate(possible_paths, 1):
    exists = path.exists()
    status = "✅" if exists else "❌"
    print(f"{status} 格式{i}: {path}")
    
    if exists:
        print(f"   ✓ 文件大小: {path.stat().st_size / 1024:.1f} KB")

print()

# 全局搜索
print("全局搜索结果:")
found = list(base_path.rglob(f"*{band}*{name}{file_extension}"))
if found:
    for f in found[:5]:
        print(f"  ✅ {f}")
else:
    print("  ❌ 未找到")
