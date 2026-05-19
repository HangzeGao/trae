#!/bin/bash
# 全面清理脚本 - 释放磁盘空间

echo "========================================"
echo "磁盘清理开始"
echo "========================================"

# 显示清理前的空间
echo -e "\n清理前空间:"
df -h /workspace

# 清理缓存
echo -e "\n清理各种缓存..."
rm -rf /root/.cache/pip/* 2>/dev/null && echo "✓ pip缓存已清理"
rm -rf /root/.cache/bazelisk/* 2>/dev/null && echo "✓ bazelisk缓存已清理"
rm -rf /root/.cache/torch/* 2>/dev/null && echo "✓ torch缓存已清理"
rm -rf /root/.cache/kagglehub/* 2>/dev/null && echo "✓ kagglehub缓存已清理"
rm -rf /root/.cache/matplotlib/* 2>/dev/null && echo "✓ matplotlib缓存已清理"

# 清理临时文件
echo -e "\n清理临时文件..."
rm -rf /tmp/* 2>/dev/null && echo "✓ /tmp已清理"
rm -rf /workspace/__pycache__ 2>/dev/null && echo "✓ pycache已清理"
rm -rf /workspace/*/__pycache__ 2>/dev/null && echo "✓ 子目录pycache已清理"
rm -f /workspace/*/*.txt 2>/dev/null && echo "✓ 日志文件已清理"

# 清理不必要的演示文件
echo -e "\n清理不必要的演示文件..."
rm -rf /workspace/demo_results 2>/dev/null && echo "✓ demo_results已清理"
rm -rf /workspace/synthetic_results 2>/dev/null && echo "✓ synthetic_results已清理"
rm -rf /workspace/data/synthetic_clouds 2>/dev/null && echo "✓ synthetic_clouds已清理"

# 清理不必要的脚本
echo -e "\n清理不必要的脚本..."
rm -f /workspace/train_synthetic.py 2>/dev/null && echo "✓ train_synthetic.py已清理"
rm -f /workspace/synthetic_training_log.txt 2>/dev/null && echo "✓ synthetic_training_log.txt已清理"
rm -f /workspace/check_syntax.py 2>/dev/null && echo "✓ check_syntax.py已清理"
rm -f /workspace/setup_and_run.sh 2>/dev/null && echo "✓ setup_and_run.sh已清理"

# 清理文档
echo -e "\n清理重复文档..."
rm -f /workspace/SKYSENSE_INTEGRATION.md 2>/dev/null && echo "✓ SKYSENSE_INTEGRATION.md已清理"
rm -f /workspace/SKYSENSEPP_38CLOUD_README.md 2>/dev/null && echo "✓ SKYSENSEPP_38CLOUD_README.md已清理"
rm -f /workspace/TRAINING_RESULTS.md 2>/dev/null && echo "✓ TRAINING_RESULTS.md已清理"

# 显示清理后的空间
echo -e "\n========================================"
echo "清理后空间:"
df -h /workspace
echo "========================================"

echo -e "\n清理完成！"
