#!/bin/bash

echo "=========================================="
echo "SkySense++ 38Cloud 训练环境设置"
echo "=========================================="
echo ""

# 首先检查是否已安装依赖
echo "检查依赖..."
pip install torch torchvision
pip install segmentation-models-pytorch
pip install kagglehub numpy pillow matplotlib tqdm pandas
pip install pyyaml scikit-learn

echo ""
echo "=========================================="
echo "依赖安装完成！"
echo "=========================================="
echo ""
echo "运行训练脚本..."
echo ""

# 运行训练
python run_skysensepp_38cloud.py
