"""
测试多数据集加载器的实际数据加载功能
"""
import torch
from data.multi_dataset_loader import get_combined_dataloader, print_dataset_summary


def test_dataloader(strategy: str, dataset_names: list):
    """测试指定策略的 DataLoader"""
    print(f"\n{'='*60}")
    print(f"测试策略: {strategy.upper()}")
    print(f"数据集: {dataset_names}")
    print(f"{'='*60}")
    
    try:
        loader = get_combined_dataloader(
            dataset_names=dataset_names,
            strategy=strategy,
            batch_size=4,
            shuffle=True,
            num_workers=0
        )
        
        print(f"✓ 创建 DataLoader 成功")
        print(f"  总样本数: {len(loader.dataset)}")
        print(f"  批次大小: {loader.batch_size}")
        print(f"  批次数: {len(loader)}")
        
        # 测试加载一个批次
        for batch_idx, (images, masks) in enumerate(loader):
            print(f"\n✓ 成功加载批次 {batch_idx + 1}")
            print(f"  图像形状: {images.shape}")
            print(f"  掩码形状: {masks.shape}")
            print(f"  图像范围: [{images.min().item():.4f}, {images.max().item():.4f}]")
            print(f"  掩码范围: [{masks.min().item():.4f}, {masks.max().item():.4f}]")
            break
        
        return True
        
    except Exception as e:
        print(f"✗ 测试失败: {e}")
        return False


def main():
    """主函数"""
    print_dataset_summary()
    
    # 测试不同策略
    strategies = ["concat", "weighted", "round_robin", "balanced"]
    datasets = ["38cloud", "95cloud"]
    
    print("\n\n开始测试多数据集加载器...")
    
    results = []
    for strategy in strategies:
        success = test_dataloader(strategy, datasets)
        results.append({"strategy": strategy, "success": success})
    
    # 总结
    print(f"\n{'='*60}")
    print("测试总结")
    print(f"{'='*60}")
    
    for result in results:
        status = "✅" if result["success"] else "❌"
        print(f"  {status} {result['strategy']}: {'通过' if result['success'] else '失败'}")
    
    all_passed = all(r["success"] for r in results)
    print(f"\n总体: {'所有测试通过！' if all_passed else '部分测试失败'}")


if __name__ == "__main__":
    main()
