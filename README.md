# Cloud Segmentation using UNet for Remote Sensing Images

This project implements a UNet-based deep learning model for semantic segmentation of clouds in remote sensing images.

## Project Overview

The goal is to accurately segment cloud regions in satellite imagery using a U-Net convolutional neural network architecture. This is useful for:
- Cloud masking in satellite image analysis
- Atmospheric monitoring
- Improving quality of remote sensing applications by removing cloud-contaminated pixels

## Architecture

**UNet** is a popular fully convolutional network designed for image segmentation tasks with:
- Symmetric encoder-decoder structure
- Skip connections to preserve spatial information
- Efficient segmentation with limited training data

## Project Structure

```
trae/
├── README.md
├── requirements.txt
├── config/
│   └── config.yaml
├── data/
│   ├── raw/
│   ├── processed/
│   └── download_data.py
├── models/
│   ├── unet.py
│   └── losses.py
├── train.py
├── evaluate.py
├── predict.py
└── notebooks/
    └── exploration.ipynb
```

## Installation

1. Clone the repository
2. Install dependencies:
```bash
pip install -r requirements.txt
```

## Usage

### Training
```bash
python train.py --config config/config.yaml
```

### Evaluation
```bash
python evaluate.py --model_path checkpoints/best_model.pth --data_dir data/processed/test
```

### Prediction
```bash
python predict.py --image_path path/to/image.tif --model_path checkpoints/best_model.pth
```

## Dataset

The project supports common remote sensing datasets:
- Sentinel-2 imagery
- Landsat-8 imagery
- CloudSEN12 dataset
- Bigearthnet dataset

## Results

TBD - Results will be updated after model training and evaluation.

## References

- U-Net: Convolutional Networks for Biomedical Image Segmentation (Ronneberger et al., 2015)
- Cloud Detection in Satellite Imagery
