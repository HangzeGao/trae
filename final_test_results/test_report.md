# SkySense++ 训练和测试报告

## 配置
- 模型: SkySense++
- 编码器: ResNet-34
- 输入通道: 4 (RGB + NIR)
- 输出: 1 (云分割)
- 融合方式: Adaptive Fusion
- 语义增强: 启用
- 模型参数: 31,365,868

## 数据集
- 总样本: 60
- 训练集: 42 样本
- 验证集: 9 样本
- 测试集: 9 样本

## 训练结果
- 最佳验证Dice: 0.6456
- 训练轮数: 10

## 测试结果
- 测试Dice: 0.5753
- 测试IoU: 0.4100

## 生成文件
- 模型: checkpoints/skysensepp_existing_best.pth
- 可视化: final_test_results/
- 报告: test_report.md

训练完成时间: /workspace
