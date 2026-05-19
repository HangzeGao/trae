import torch
import torch.nn as nn
import torch.nn.functional as F
from typing import Optional

try:
    from utils.config import Config
    HAS_CONFIG = True
except ImportError:
    HAS_CONFIG = False


class DiceLoss(nn.Module):
    """Dice Loss for segmentation"""
    
    def __init__(self, smooth=1e-6):
        super(DiceLoss, self).__init__()
        self.smooth = smooth
    
    def forward(self, predictions, targets):
        predictions = predictions.view(-1)
        targets = targets.view(-1)
        
        intersection = (predictions * targets).sum()
        dice = (2.0 * intersection + self.smooth) / (predictions.sum() + targets.sum() + self.smooth)
        return 1.0 - dice


class BCEWithDiceLoss(nn.Module):
    """Combination of Binary Cross Entropy and Dice Loss"""
    
    def __init__(self, bce_weight=0.5, dice_weight=0.5):
        super(BCEWithDiceLoss, self).__init__()
        self.bce_weight = bce_weight
        self.dice_weight = dice_weight
        self.bce_loss = nn.BCELoss()
        self.dice_loss = DiceLoss()
    
    def forward(self, predictions, targets):
        bce = self.bce_loss(predictions, targets)
        dice = self.dice_loss(predictions, targets)
        return self.bce_weight * bce + self.dice_weight * dice


class FocalLoss(nn.Module):
    """Focal Loss for handling class imbalance"""
    
    def __init__(self, alpha=0.25, gamma=2.0):
        super(FocalLoss, self).__init__()
        self.alpha = alpha
        self.gamma = gamma
    
    def forward(self, predictions, targets):
        bce = F.binary_cross_entropy(predictions, targets, reduction='none')
        p_t = predictions * targets + (1 - predictions) * (1 - targets)
        focal_weight = (1 - p_t) ** self.gamma
        focal_loss = self.alpha * focal_weight * bce
        return focal_loss.mean()


class IoULoss(nn.Module):
    """Intersection over Union Loss"""
    
    def __init__(self, smooth=1e-6):
        super(IoULoss, self).__init__()
        self.smooth = smooth
    
    def forward(self, predictions, targets):
        predictions = predictions.view(-1)
        targets = targets.view(-1)
        
        intersection = (predictions * targets).sum()
        union = predictions.sum() + targets.sum() - intersection
        iou = (intersection + self.smooth) / (union + self.smooth)
        return 1.0 - iou


def get_loss_function_from_config(config: Config, **kwargs) -> nn.Module:
    """
    Get loss function from Config object.
    
    Args:
        config: Config object
        **kwargs: Additional loss kwargs
    
    Returns:
        Loss function module
    """
    loss_name = config.training.get('loss', 'dice_bce')
    return get_loss_function(loss_name, **kwargs)


def get_loss_function(loss_name="dice_bce", **kwargs) -> nn.Module:
    """
    Get loss function by name.
    
    Args:
        loss_name: Name of loss function
        **kwargs: Additional parameters for loss
    
    Returns:
        Loss function module
    """
    
    loss_dict = {
        "bce": nn.BCELoss(),
        "dice": DiceLoss(),
        "dice_bce": BCEWithDiceLoss(**kwargs),
        "focal": FocalLoss(**kwargs),
        "iou": IoULoss(),
    }
    
    if loss_name not in loss_dict:
        raise ValueError(f"Unknown loss function: {loss_name}")
    
    return loss_dict[loss_name]
