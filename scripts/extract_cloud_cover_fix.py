"""
修复提取 Cloud Cover Detection 数据集
"""
import subprocess
import time
from pathlib import Path

def extract_dataset():
    """使用系统命令提取数据集"""
    base_path = Path("/root/.cache/kagglehub/datasets/hmendonca/cloud-cover-detection")
    archive_path = base_path / "3.archive"
    extract_path = base_path / "extracted"
    
    print("=" * 60)
    print("提取 Cloud Cover Detection 数据集")
    print("=" * 60)
    
    if not archive_path.exists():
        print(f"❌ archive 文件不存在: {archive_path}")
        return None
    
    print(f"Archive 文件: {archive_path}")
    print(f"文件大小: {archive_path.stat().st_size / (1024 * 1024 * 1024):.2f} GB")
    
    extract_path.mkdir(parents=True, exist_ok=True)
    
    start_time = time.time()
    
    try:
        # 使用 unzip 命令提取
        print("\n使用 unzip 命令提取...")
        result = subprocess.run(
            ["unzip", str(archive_path), "-d", str(extract_path)],
            capture_output=True,
            text=True
        )
        
        if result.returncode == 0:
            print("✅ 提取成功！")
        else:
            print(f"❌ unzip 失败: {result.stderr}")
            
            # 尝试使用 7z
            print("\n尝试使用 7z...")
            result = subprocess.run(
                ["7z", "x", str(archive_path), f"-o{extract_path}"],
                capture_output=True,
                text=True
            )
            
            if result.returncode == 0:
                print("✅ 7z 提取成功！")
            else:
                print(f"❌ 7z 也失败了: {result.stderr}")
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
