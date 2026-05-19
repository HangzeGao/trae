
import numpy as np
from pathlib import Path
from PIL import Image

# 查找有值的 gt 样本
base_path = Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images/versions/4")
gt_dir = base_path / "38-Cloud_training/train_gt"

count = 0
for gt_file in gt_dir.glob("*.TIF"):
    gt = np.array(Image.open(gt_file))
    if gt.max() > 0:
        print(f"找到有值的样本: {gt_file.name}")
        print(f"GT 唯一值: {np.unique(gt)}")
        print(f"非零像素比例: {(gt > 0).mean():.4f}")
        count += 1
        if count >= 3:  # 只找前 3 个
            break

print(f"\n找到了 {count} 个有值的样本")
