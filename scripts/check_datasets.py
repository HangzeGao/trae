"""
检查所有三个 Kaggle 数据集的下载状态
"""
from pathlib import Path


def check_dataset(dataset_name: str, path: Path) -> dict:
    """检查单个数据集"""
    result = {
        'name': dataset_name,
        'path': str(path),
        'exists': False,
        'has_files': False,
        'file_count': 0,
        'total_size_mb': 0,
        'status': '未下载'
    }
    
    if not path.exists():
        return result
    
    result['exists'] = True
    
    # 查找所有文件
    all_files = list(path.rglob('*'))
    files = [f for f in all_files if f.is_file()]
    
    if files:
        result['has_files'] = True
        result['file_count'] = len(files)
        result['total_size_mb'] = sum(f.stat().st_size for f in files) / (1024 ** 2)
        
        # 判断状态
        if any('.TIF' in str(f).upper() for f in files):
            result['status'] = '已完成'
        elif any('.archive' in str(f).lower() for f in files):
            result['status'] = '下载中/待解压'
        else:
            result['status'] = '部分下载'
    
    return result


def main():
    print("=" * 70)
    print("检查 Kaggle 数据集下载状态")
    print("=" * 70)
    
    datasets = [
        {
            'name': '38-Cloud',
            'id': 'sorour/38cloud-cloud-segmentation-in-satellite-images',
            'path': Path("/root/.cache/kagglehub/datasets/sorour/38cloud-cloud-segmentation-in-satellite-images")
        },
        {
            'name': '95-Cloud',
            'id': 'sorour/95cloud-cloud-segmentation-on-satellite-images',
            'path': Path("/root/.cache/kagglehub/datasets/sorour/95cloud-cloud-segmentation-on-satellite-images")
        },
        {
            'name': 'Cloud Cover Detection',
            'id': 'hmendonca/cloud-cover-detection',
            'path': Path("/root/.cache/kagglehub/datasets/hmendonca/cloud-cover-detection")
        }
    ]
    
    print(f"\n{'数据集':<25} {'状态':<12} {'文件数':<8} {'大小(MB)':<12} {'路径'}")
    print("-" * 70)
    
    for ds in datasets:
        info = check_dataset(ds['name'], ds['path'])
        
        size_str = f"{info['total_size_mb']:.1f}" if info['total_size_mb'] > 0 else '-'
        count_str = str(info['file_count']) if info['file_count'] > 0 else '-'
        
        print(f"{info['name']:<25} {info['status']:<12} {count_str:<8} {size_str:<12} {info['path'][:50]}...")
    
    print("\n" + "=" * 70)
    
    # 汇总
    completed = sum(1 for ds in datasets if check_dataset(ds['name'], ds['path'])['status'] == '已完成')
    downloading = sum(1 for ds in datasets if check_dataset(ds['name'], ds['path'])['status'] == '下载中/待解压')
    partial = sum(1 for ds in datasets if check_dataset(ds['name'], ds['path'])['status'] == '部分下载')
    
    print(f"\n汇总:")
    print(f"  ✓ 已完成: {completed} 个")
    print(f"  ⏳ 下载中: {downloading} 个")
    print(f"  ⚠️ 部分下载: {partial} 个")
    print(f"  ✗ 未下载: {len(datasets) - completed - downloading - partial} 个")
    
    print("\n" + "=" * 70)


if __name__ == "__main__":
    main()
