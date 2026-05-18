"""
Dataset preparation utilities
"""

import os
import argparse
from pathlib import Path


def setup_directories(base_dir='./data'):
    """Create directory structure for project"""
    
    dirs = [
        'raw',
        'processed/train/images',
        'processed/train/masks',
        'processed/val/images',
        'processed/val/masks',
        'processed/test/images',
        'processed/test/masks',
    ]
    
    for dir_name in dirs:
        path = os.path.join(base_dir, dir_name)
        Path(path).mkdir(parents=True, exist_ok=True)
        print(f"✓ Created {path}")


def print_dataset_info():
    """Print information about supported datasets"""
    
    info = """
    Supported Remote Sensing Datasets for Cloud Segmentation
    =========================================================
    
    1. CloudSEN12 Dataset
       - Multi-spectral Sentinel-2 imagery
       - Cloud and shadow segmentation masks
       - Download: https://github.com/cloudsen12/cloudsen12.github.io
    
    2. Bigearthnet Dataset
       - Multi-spectral Sentinel-2 imagery
       - Various land cover annotations
       - Download: http://bigearth.net/
    
    3. Sentinel-2 Imagery (Custom)
       - Free satellite imagery from Copernicus
       - Download: https://scihub.copernicus.eu/
       - Process using GDAL to create cloud masks
    
    4. Landsat-8 Imagery (Custom)
       - Free satellite imagery from USGS
       - Download: https://earthexplorer.usgs.gov/
       - Use built-in quality assessment bands for cloud detection
    
    Expected Data Format
    ====================
    Place your data in the following structure:
    
    data/
    ├── raw/
    │   ├── <dataset_name>/
    │   │   ├── images/
    │   │   └── masks/
    │   └── ...
    └── processed/
        ├── train/
        │   ├── images/
        │   └── masks/
        ├── val/
        │   ├── images/
        │   └── masks/
        └── test/
            ├── images/
            └── masks/
    
    Image Format
    ============
    - Supported formats: PNG, JPG, JPEG, TIF, TIFF
    - Size: Any (will be resized to 256x256 during training)
    - For Sentinel-2: Either RGB composite or multi-spectral bands
    
    Mask Format
    ===========
    - Format: PNG grayscale
    - Values: 0 (no cloud), 255 (cloud)
    - Size: Same as corresponding image
    """
    
    print(info)


def main():
    parser = argparse.ArgumentParser(description='Dataset preparation utilities')
    parser.add_argument('--setup_dirs', action='store_true', help='Create directory structure')
    parser.add_argument('--info', action='store_true', help='Print dataset information')
    parser.add_argument('--data_dir', type=str, default='./data', help='Base data directory')
    
    args = parser.parse_args()
    
    if args.setup_dirs:
        setup_directories(args.data_dir)
        print("\n✓ Directory structure created successfully!")
    elif args.info:
        print_dataset_info()
    else:
        parser.print_help()


if __name__ == '__main__':
    main()
