# 38Cloud 遥感云分割数据集下载和使用指南

## 📋 任务概述
1. 尝试下载38Cloud公开遥感云分割数据集
2. 如果下载速度过慢，使用小规模替代数据集

---

## 📦 38Cloud 数据集详情

### 数据集介绍
38-Cloud 是一个用于云检测的云分割数据集，包含了38个Landsat 8场景图像及其手动提取的像素级地面真实值。

### 数据集规格
- **图像来源**: Landsat 8
- **图像大小**: 384×384 像素补丁
- **训练样本**: 8,400 个补丁
- **测试样本**: 9,201 个补丁
- **光谱通道**: 4个通道（Red, Green, Blue, Near Infrared）
- **数据格式**: .TIF

### 通道配置
- Red (波段 4)
- Green (波段 3) 
- Blue (波段 2)
- Near Infrared (NIR, 波段 5)

---

## 📥 完整数据集下载方式

### 方式1: Kaggle下载
```bash
# 使用Python的kagglehub库
pip install kagglehub

import kagglehub
path = kagglehub.dataset_download("sorour/38cloud-cloud-segmentation-in-satellite-images")
print("Path to dataset files:", path)
```

**Kaggle链接**: https://www.kaggle.com/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images

### 方式2: Google Drive下载
**Google Drive链接**: https://goo.gl/683SHf

### 方式3: GitHub仓库克隆
```bash
git clone https://github.com/SorourMo/38-Cloud-A-Cloud-Segmentation-Dataset.git
```

**GitHub链接**: https://github.com/SorourMo/38-Cloud-A-Cloud-Segmentation-Dataset

### 扩展数据集: 95-Cloud
95-Cloud是38-Cloud的扩展版本，包含95个Landsat 8图像
**GitHub链接**: https://github.com/SorourMo/95-Cloud-An-Extension-to-38-Cloud-Dataset

---

## 🔄 小规模替代数据集

由于38Cloud数据集体积较大（12GB+），以下是合适的小规模替代数据集：

### 1. L8_SPARCS (推荐)
- **样本数**: 80个场景子集（1000×1000像素）
- **来源**: Landsat-8 (30m)
- **标注**: 云和云阴影
- **下载链接**: https://www.usgs.gov/core-science-systems/nli/landsat/spatial-procedures-automated-removal-cloud-and-shadow-sparcs

### 2. L7_Irish
- **样本数**: 206个Landsat-7场景
- **来源**: Landsat-7 (30m)
- **标注**: 云（部分含云阴影）
- **下载链接**: https://landsat.usgs.gov/landsat-7-cloud-cover-assessment-validation-data

### 3. L8_Biome
- **样本数**: 96个Landsat-8场景（8个生物群落）
- **来源**: Landsat-8 (30m)
- **标注**: 云（部分含云阴影）
- **下载链接**: https://landsat.usgs.gov/landsat-8-cloud-cover-assessment-validation-data

### 4. WHU Cloud Dataset
- **样本数**: 7个Landsat-8图像（6个不同地区）
- **特点**: 含无云历史图像和云阴影标注
- **下载链接**: http://gpcv.whu.edu.cn/data/WHU_Cloud_Dataset.html

### 5. RICE_dataset
- **样本数**: 450个Landsat-8图像（512×512像素）
- **特点**: 含无云图像和云标签
- **GitHub链接**: https://github.com/BUPTLdy/RICE_DATASET

---

## 📊 云分割数据集对比表

| 数据集 | 卫星 | 样本数 | 标注类型 | 大小 | 特点 |
|-------|------|--------|---------|------|------|
| **38-Cloud** | Landsat-8 | 8,400训练/9,201测试 | 云 | ~12GB | 4通道(RGB+NIR) |
| **95-Cloud** | Landsat-8 | 38→95扩展 | 云 | 更大 | 38-Cloud的扩展版 |
| **L8_SPARCS** | Landsat-8 | 80个 | 云+云阴影 | 小 | 含云阴影标注 |
| **L7_Irish** | Landsat-7 | 206个 | 云(部分阴影) | 小 | 9个纬度区 |
| **L8_Biome** | Landsat-8 | 96个 | 云(部分阴影) | 小 | 8个生物群落 |
| **WHU Cloud** | Landsat-8 | 7个 | 云+云阴影 | 小 | 含无云历史图像 |
| **RICE** | Landsat-8 | 450个 | 云 | 中等 | 含无云对照图像 |

---

## 💻 快速开始指南

### 使用38Cloud数据集
```python
import numpy as np
from PIL import Image
import os

# 加载训练数据
train_dir = '38-Cloud_training'
train_red = np.array([np.array(Image.open(os.path.join(train_dir, 'train_red', img))) 
                     for img in os.listdir(os.path.join(train_dir, 'train_red'))])
train_green = np.array([np.array(Image.open(os.path.join(train_dir, 'train_green', img))) 
                       for img in os.listdir(os.path.join(train_dir, 'train_green'))])
train_blue = np.array([np.array(Image.open(os.path.join(train_dir, 'train_blue', img))) 
                      for img in os.listdir(os.path.join(train_dir, 'train_blue'))])
train_nir = np.array([np.array(Image.open(os.path.join(train_dir, 'train_nir', img))) 
                     for img in os.listdir(os.path.join(train_dir, 'train_nir'))])

# 合并通道
train_data = np.stack((train_red, train_green, train_blue, train_nir), axis=-1)
```

---

## 📚 相关论文
1. Mohajerani, S., & Saeedi, P. (2019). 38-Cloud: A Cloud Segmentation Dataset. arXiv:1901.10077
2. 38-Cloud数据集原始论文: https://arxiv.org/pdf/1901.10077.pdf

---

## 📝 总结
根据我们的环境情况（磁盘空间有限），建议：
1. **优先使用已有数据**: `data/clouds_for_training` 目录中的60个样本
2. **或尝试下载L8_SPARCS**: 80个样本适合快速实验
3. **如需真实训练**: 准备足够磁盘空间后下载38Cloud完整数据集

---

**参考来源**:
- 38-Cloud GitHub: https://github.com/SorourMo/38-Cloud-A-Cloud-Segmentation-Dataset
- 云检测数据集汇总: https://github.com/dr-lizhiwei/OpenSICDR
- CSDN教程: https://blog.csdn.net/gitblog_00942/article/details/141083876
