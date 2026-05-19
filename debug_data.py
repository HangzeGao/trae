
import numpy as np
from pathlib import Path
from PIL import Image

# 检查一下第一个样本
base_path = Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4")
split_dir = base_path / "38-Cloud_training"

name = "patch_10_1_by_10_LC08_L1TP_002053_20160520_20170324_01_T1"

blue_path = split_dir / "train_blue" / f"blue_{name}.TIF"
gt_path = split_dir / "train_gt" / f"gt_{name}.TIF"

print(f"检查 {name} 的数据:")

# 加载并打印图像统计信息
blue = np.array(Image.open(blue_path))
print(f"Blue 图像形状: {blue.shape}, 数据类型: {blue.dtype}")
print(f"Blue 图像 最小值: {blue.min()}, 最大值: {blue.max()}, 均值: {blue.mean():.2f}")

# 加载并打印掩码信息
gt = np.array(Image.open(gt_path))
print(f"\nGT 掩码形状: {gt.shape}, 数据类型: {gt.dtype}")
print(f"GT 掩码 唯一值: {np.unique(gt)}")
print(f"GT 掩码 非零像素比例: {(gt > 0).mean():.4f}")

# 检查其他波段
green_path = split_dir / "train_green" / f"green_{name}.TIF"
red_path = split_dir / "train_red" / f"red_{name}.TIF"
nir_path = split_dir / "train_nir" / f"nir_{name}.TIF"

green = np.array(Image.open(green_path))
red = np.array(Image.open(red_path))
nir = np.array(Image.open(nir_path))

print(f"\n其他波段:")
print(f"Green: min={green.min()}, max={green.max()}, mean={green.mean():.2f}")
print(f"Red: min={red.min()}, max={red.max()}, mean={red.mean():.2f}")
print(f"NIR: min={nir.min()}, max={nir.max()}, mean={nir.mean():.2f}")
