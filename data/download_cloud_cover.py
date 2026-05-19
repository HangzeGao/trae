"""
下载 Cloud Cover Detection 数据集
"""
import kagglehub
from pathlib import Path


def download_cloud_cover_dataset():
    """下载 Cloud Cover Detection 数据集"""
    
    print("=" * 60)
    print("下载 Cloud Cover Detection 数据集")
    print("=" * 60)
    
    dataset_id = "hmendonca/cloud-cover-detection"
    
    print(f"\n正在下载数据集: {dataset_id}")
    print("这可能需要几分钟时间，请耐心等待...\n")
    
    try:
        # 下载数据集
        path = kagglehub.dataset_download(dataset_id)
        
        print(f"\n✓ 下载成功！")
        print(f"路径: {path}")
        
        # 验证路径
        path_obj = Path(path)
        if path_obj.exists():
            print(f"\n数据集统计:")
            print(f"  目录存在: ✓")
            print(f"  总大小: {sum(f.stat().st_size for f in path_obj.rglob('*') if f.is_file()) / (1024**2):.1f} MB")
            
            # 显示文件列表
            print(f"\n文件结构预览:")
            for i, item in enumerate(sorted(path_obj.rglob('*'))[:20]):
                if item.is_file():
                    print(f"  {item.relative_to(path_obj)}")
            if len(list(path_obj.rglob('*'))) > 20:
                print(f"  ... 共 {len(list(path_obj.rglob('*')))} 个文件")
        
        return path
        
    except Exception as e:
        print(f"\n✗ 下载失败: {e}")
        print("\n可能的原因:")
        print("  1. 网络连接不稳定")
        print("  2. Kaggle API 未正确配置")
        print("  3. 数据集较大，下载超时")
        print("\n建议:")
        print("  1. 检查网络连接")
        print("  2. 手动使用 kagglehub 下载:")
        print(f"     import kagglehub")
        print(f"     path = kagglehub.dataset_download('{dataset_id}')")
        print(f"     print(path)")
        return None


if __name__ == "__main__":
    download_cloud_cover_dataset()
