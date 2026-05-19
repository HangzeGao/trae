"""
手动提取 Cloud Cover Detection 数据集
"""
import zipfile
import tarfile
import time
from pathlib import Path

def extract_dataset():
    """手动提取数据集"""
    base_path = Path("/root/.cache/kagglehub/datasets/hmendonca/cloud-cover-detection")
    archive_path = base_path / "3.archive"
    extract_path = base_path / "extracted"
    
    print("=" * 60)
    print("手动提取 Cloud Cover Detection 数据集")
    print("=" * 60)
    
    # 检查 archive 文件
    if not archive_path.exists():
        print(f"❌ archive 文件不存在: {archive_path}")
        return None
    
    print(f"Archive 文件: {archive_path}")
    print(f"文件大小: {archive_path.stat().st_size / (1024 * 1024 * 1024):.2f} GB")
    
    # 创建提取目录
    extract_path.mkdir(parents=True, exist_ok=True)
    
    start_time = time.time()
    
    try:
        # 尝试多种格式
        print("\n尝试提取...")
        
        # 尝试作为 zip 文件
        try:
            with zipfile.ZipFile(archive_path, 'r') as zip_ref:
                zip_ref.extractall(extract_path)
                print("✅ 作为 ZIP 文件提取成功")
        except zipfile.BadZipFile:
            # 尝试作为 tar.gz 文件
            try:
                with tarfile.open(archive_path, 'r:gz') as tar_ref:
                    tar_ref.extractall(extract_path)
                    print("✅ 作为 TAR.GZ 文件提取成功")
            except Exception:
                # 尝试作为普通 tar 文件
                try:
                    with tarfile.open(archive_path, 'r') as tar_ref:
                        tar_ref.extractall(extract_path)
                        print("✅ 作为 TAR 文件提取成功")
                except Exception as e:
                    print(f"❌ 无法识别文件格式: {e}")
                    return None
        
        elapsed = time.time() - start_time
        print(f"\n提取耗时: {elapsed:.1f} 秒")
        
        # 检查提取的文件
        files = list(extract_path.rglob("*"))
        print(f"\n提取的文件数: {len(files)}")
        
        # 显示目录结构
        print("\n目录结构:")
        for item in sorted(extract_path.iterdir()):
            if item.is_dir():
                sub_files = list(item.rglob("*"))
                print(f"  📂 {item.name} ({len(sub_files)} 个文件)")
            else:
                print(f"  📄 {item.name} ({item.stat().st_size / 1024:.1f} KB)")
        
        return extract_path
        
    except Exception as e:
        print(f"\n❌ 提取失败: {e}")
        import traceback
        traceback.print_exc()
        return None


if __name__ == "__main__":
    extract_dataset()
