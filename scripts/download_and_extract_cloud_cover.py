"""
下载并提取 Cloud Cover Detection 数据集
"""
import kagglehub
import time
from pathlib import Path

def download_and_extract():
    """下载并提取数据集"""
    dataset_id = "hmendonca/cloud-cover-detection"
    
    print("=" * 60)
    print(f"处理数据集: {dataset_id}")
    print("=" * 60)
    
    try:
        print("开始下载/提取...")
        start_time = time.time()
        
        # 强制重新下载/提取
        path = kagglehub.dataset_download(
            dataset_id, 
            force_download=True,
            unzip=True
        )
        
        elapsed = time.time() - start_time
        
        print(f"\n✅ 完成！")
        print(f"路径: {path}")
        print(f"耗时: {elapsed:.1f} 秒")
        
        # 检查提取的文件
        path_obj = Path(path)
        if path_obj.exists():
            files = list(path_obj.rglob("*"))
            print(f"\n提取的文件数: {len(files)}")
            
            # 显示前几个文件
            print("\n部分文件:")
            for f in files[:10]:
                size_mb = f.stat().st_size / (1024 * 1024) if f.is_file() else "目录"
                print(f"  {f.relative_to(path_obj)} - {size_mb:.2f} MB" if isinstance(size_mb, float) else f"  {f.relative_to(path_obj)} - {size_mb}")
        
        return path
        
    except Exception as e:
        print(f"\n❌ 失败: {e}")
        import traceback
        traceback.print_exc()
        return None


if __name__ == "__main__":
    download_and_extract()
