"""
下载 Cloud Cover Detection 数据集
"""
import kagglehub
import time

def download_dataset():
    """下载数据集"""
    dataset_id = "hmendonca/cloud-cover-detection"
    
    print("=" * 60)
    print(f"开始下载: {dataset_id}")
    print("=" * 60)
    print("数据集大小: 约 16 GB")
    print("预计下载时间: 取决于网络速度")
    print("=" * 60)
    
    try:
        # 开始下载（支持断点续传）
        start_time = time.time()
        path = kagglehub.dataset_download(dataset_id, force_download=True)
        
        elapsed = time.time() - start_time
        print(f"\n✅ 下载完成！")
        print(f"路径: {path}")
        print(f"耗时: {elapsed:.1f} 秒")
        
        return path
        
    except Exception as e:
        print(f"\n❌ 下载失败: {e}")
        print("\n可能的解决方案:")
        print("1. 检查网络连接")
        print("2. 稍等片刻后重试")
        print("3. 确保有足够的磁盘空间")
        return None


if __name__ == "__main__":
    download_dataset()
