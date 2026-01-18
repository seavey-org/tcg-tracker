#!/usr/bin/env python3
"""
Generate synthetic dataset for card localization training.

This script creates synthetic training images by pasting card images onto
random table/background images with various augmentations (rotation, scaling,
perspective transforms, lighting changes).

The output is suitable for training YOLOv8-Nano for card detection/segmentation.

Usage:
    python -m identifier.scripts.generate_synthetic_dataset \
        --cards-dir ./data/card_images \
        --backgrounds-dir ./data/backgrounds \
        --output-dir ./data/synthetic_dataset \
        --num-images 10000

Output format (YOLO segmentation):
    output_dir/
        images/
            train/
            val/
        labels/
            train/
            val/

Environment variables:
    POKEMONTCG_API_KEY - For downloading card images from API (optional)
"""

import argparse
import logging
import random
import sys
from pathlib import Path

import cv2
import numpy as np

logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")
logger = logging.getLogger(__name__)

# Standard card aspect ratio (width/height)
CARD_ASPECT_RATIO = 0.714  # ~63mm x 88mm


def download_sample_backgrounds(output_dir: Path, count: int = 20) -> list[Path]:
    """Download sample background images (table textures, surfaces)."""
    from urllib.request import Request, urlopen
    from urllib.error import HTTPError, URLError

    # URLs for various table/surface textures (creative commons or public domain)
    # In production, you'd use your own background images
    background_urls = [
        # Wood textures
        "https://images.unsplash.com/photo-1558618666-fcd25c85cd64?w=800",  # wood table
        "https://images.unsplash.com/photo-1541123603104-512919d6a96c?w=800",  # dark wood
        "https://images.unsplash.com/photo-1549619856-ac562a3ed1a3?w=800",  # light wood
        # Cloth/felt textures
        "https://images.unsplash.com/photo-1531685250784-7569952593d2?w=800",  # green felt
        "https://images.unsplash.com/photo-1553095066-5014bc7b7f2d?w=800",  # neutral fabric
    ]

    output_dir.mkdir(parents=True, exist_ok=True)
    downloaded = []

    for i, url in enumerate(background_urls[:count]):
        output_path = output_dir / f"background_{i:03d}.jpg"
        if output_path.exists():
            downloaded.append(output_path)
            continue

        try:
            req = Request(url, headers={"User-Agent": "TCGScanner/1.0"})
            with urlopen(req, timeout=30) as response:
                data = response.read()
                with open(output_path, "wb") as f:
                    f.write(data)
                downloaded.append(output_path)
                logger.info(f"Downloaded background {i+1}/{len(background_urls)}")
        except (HTTPError, URLError) as e:
            logger.warning(f"Failed to download background {i}: {e}")

    return downloaded


def generate_solid_backgrounds(output_dir: Path, count: int = 10) -> list[Path]:
    """Generate solid color backgrounds (common table colors)."""
    output_dir.mkdir(parents=True, exist_ok=True)
    generated = []

    # Common table/playmat colors
    colors = [
        (34, 34, 34),     # Dark gray
        (64, 64, 64),     # Medium gray
        (128, 128, 128),  # Light gray
        (20, 60, 20),     # Dark green (playmat)
        (30, 30, 80),     # Dark blue
        (60, 40, 30),     # Brown (wood-like)
        (180, 180, 170),  # Off-white
        (200, 190, 180),  # Beige
        (40, 40, 60),     # Dark blue-gray
        (80, 60, 50),     # Warm brown
    ]

    for i, color in enumerate(colors[:count]):
        output_path = output_dir / f"solid_{i:03d}.jpg"
        if output_path.exists():
            generated.append(output_path)
            continue

        # Create solid color image with some noise for realism
        img = np.full((1080, 1920, 3), color, dtype=np.uint8)
        noise = np.random.randint(-10, 10, img.shape, dtype=np.int16)
        img = np.clip(img.astype(np.int16) + noise, 0, 255).astype(np.uint8)

        cv2.imwrite(str(output_path), img)
        generated.append(output_path)

    return generated


def random_perspective_transform(
    card_img: np.ndarray,
    max_rotation: float = 30,
    max_perspective: float = 0.1,
    scale_range: tuple[float, float] = (0.3, 0.8),
) -> tuple[np.ndarray, np.ndarray]:
    """Apply random perspective transform to card image.

    Returns:
        Transformed image and the 4 corner points (for segmentation mask)
    """
    h, w = card_img.shape[:2]

    # Random rotation
    angle = random.uniform(-max_rotation, max_rotation)

    # Random scale
    scale = random.uniform(*scale_range)
    _ = int(w * scale)  # new_w, unused but kept for symmetry
    _ = int(h * scale)  # new_h, unused but kept for symmetry

    # Source points (corners of original card)
    src_pts = np.float32([
        [0, 0],
        [w, 0],
        [w, h],
        [0, h],
    ])

    # Apply perspective distortion
    dst_pts = src_pts.copy()
    for i in range(4):
        dst_pts[i, 0] += random.uniform(-w * max_perspective, w * max_perspective)
        dst_pts[i, 1] += random.uniform(-h * max_perspective, h * max_perspective)

    # Get perspective transform matrix
    M = cv2.getPerspectiveTransform(src_pts, dst_pts)

    # Calculate output size
    corners = np.float32([[0, 0], [w, 0], [w, h], [0, h]]).reshape(-1, 1, 2)
    transformed_corners = cv2.perspectiveTransform(corners, M)
    x_min = int(transformed_corners[:, 0, 0].min())
    y_min = int(transformed_corners[:, 0, 1].min())
    x_max = int(transformed_corners[:, 0, 0].max())
    y_max = int(transformed_corners[:, 0, 1].max())

    # Adjust transform to fit in positive coordinates
    translation = np.float32([
        [1, 0, -x_min],
        [0, 1, -y_min],
        [0, 0, 1],
    ])
    M = translation @ M

    out_w = x_max - x_min
    out_h = y_max - y_min

    # Apply rotation
    center = (out_w / 2, out_h / 2)
    rot_matrix = cv2.getRotationMatrix2D(center, angle, 1.0)

    # Extend to 3x3
    rot_3x3 = np.eye(3)
    rot_3x3[:2, :] = rot_matrix

    # Combine transforms
    M = rot_3x3 @ M

    # Scale
    scale_matrix = np.float32([
        [scale, 0, 0],
        [0, scale, 0],
        [0, 0, 1],
    ])
    M = scale_matrix @ M

    # Calculate final output size
    final_w = int(out_w * scale)
    final_h = int(out_h * scale)

    # Apply transform
    transformed = cv2.warpPerspective(
        card_img, M, (final_w, final_h),
        borderMode=cv2.BORDER_CONSTANT,
        borderValue=(0, 0, 0, 0) if card_img.shape[2] == 4 else (0, 0, 0),
    )

    # Get transformed corner points
    corners = np.float32([[0, 0, 1], [w, 0, 1], [w, h, 1], [0, h, 1]])
    new_corners = []
    for c in corners:
        nc = M @ c
        new_corners.append([nc[0] / nc[2], nc[1] / nc[2]])
    new_corners = np.array(new_corners, dtype=np.float32)

    return transformed, new_corners


def apply_lighting_augmentation(img: np.ndarray) -> np.ndarray:
    """Apply random lighting/color augmentations."""
    # Random brightness
    brightness = random.uniform(0.7, 1.3)
    img = np.clip(img * brightness, 0, 255).astype(np.uint8)

    # Random contrast
    contrast = random.uniform(0.8, 1.2)
    mean = np.mean(img)
    img = np.clip((img - mean) * contrast + mean, 0, 255).astype(np.uint8)

    # Random saturation (in HSV space)
    if random.random() > 0.5:
        hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV).astype(np.float32)
        saturation = random.uniform(0.8, 1.2)
        hsv[:, :, 1] = np.clip(hsv[:, :, 1] * saturation, 0, 255)
        img = cv2.cvtColor(hsv.astype(np.uint8), cv2.COLOR_HSV2BGR)

    return img


def add_shadow(img: np.ndarray, mask: np.ndarray) -> np.ndarray:
    """Add a subtle shadow under the card."""
    # Create shadow mask (offset and blurred version of card mask)
    shadow_offset = (random.randint(5, 20), random.randint(5, 20))
    shadow_mask = np.roll(mask, shadow_offset[0], axis=0)
    shadow_mask = np.roll(shadow_mask, shadow_offset[1], axis=1)
    shadow_mask = cv2.GaussianBlur(shadow_mask, (21, 21), 10)

    # Apply shadow (darken background)
    shadow_intensity = random.uniform(0.2, 0.4)
    shadow_layer = (shadow_mask / 255.0 * shadow_intensity)[:, :, np.newaxis]
    img = np.clip(img * (1 - shadow_layer), 0, 255).astype(np.uint8)

    return img


def paste_card_on_background(
    card_img: np.ndarray,
    background: np.ndarray,
    position: tuple[int, int] | None = None,
) -> tuple[np.ndarray, np.ndarray, list[tuple[float, float]]]:
    """Paste a transformed card onto a background.

    Args:
        card_img: Card image (BGR or BGRA)
        background: Background image (BGR)
        position: Optional (x, y) position, otherwise random

    Returns:
        - Result image
        - Segmentation mask
        - Corner points (normalized 0-1)
    """
    bg_h, bg_w = background.shape[:2]
    card_h, card_w = card_img.shape[:2]

    # Random position if not specified
    if position is None:
        max_x = bg_w - card_w
        max_y = bg_h - card_h
        if max_x < 0 or max_y < 0:
            # Card too big, scale down
            scale = min(bg_w / card_w, bg_h / card_h) * 0.8
            card_img = cv2.resize(card_img, None, fx=scale, fy=scale)
            card_h, card_w = card_img.shape[:2]
            max_x = bg_w - card_w
            max_y = bg_h - card_h

        x = random.randint(max(0, max_x // 4), max(1, max_x * 3 // 4))
        y = random.randint(max(0, max_y // 4), max(1, max_y * 3 // 4))
    else:
        x, y = position

    # Ensure card fits
    x = max(0, min(x, bg_w - card_w))
    y = max(0, min(y, bg_h - card_h))

    # Create output
    result = background.copy()
    mask = np.zeros((bg_h, bg_w), dtype=np.uint8)

    # Handle alpha channel if present
    if card_img.shape[2] == 4:
        alpha = card_img[:, :, 3] / 255.0
        card_rgb = card_img[:, :, :3]

        # Blend card onto background
        roi = result[y:y+card_h, x:x+card_w]
        for c in range(3):
            roi[:, :, c] = (alpha * card_rgb[:, :, c] + (1 - alpha) * roi[:, :, c]).astype(np.uint8)

        # Create mask
        mask[y:y+card_h, x:x+card_w] = (alpha * 255).astype(np.uint8)
    else:
        result[y:y+card_h, x:x+card_w] = card_img
        mask[y:y+card_h, x:x+card_w] = 255

    # Calculate corner points (normalized)
    corners = [
        (x / bg_w, y / bg_h),
        ((x + card_w) / bg_w, y / bg_h),
        ((x + card_w) / bg_w, (y + card_h) / bg_h),
        (x / bg_w, (y + card_h) / bg_h),
    ]

    return result, mask, corners


def create_yolo_label(corners: list[tuple[float, float]], class_id: int = 0) -> str:
    """Create YOLO segmentation format label.

    Format: class_id x1 y1 x2 y2 x3 y3 x4 y4
    (All coordinates normalized 0-1)
    """
    points = " ".join([f"{x:.6f} {y:.6f}" for x, y in corners])
    return f"{class_id} {points}"


def generate_synthetic_image(
    card_paths: list[Path],
    background_paths: list[Path],
    output_size: tuple[int, int] = (640, 640),
    num_cards: int = 1,
) -> tuple[np.ndarray, list[str]]:
    """Generate a single synthetic training image.

    Args:
        card_paths: List of card image paths to sample from
        background_paths: List of background image paths
        output_size: Output image size (width, height)
        num_cards: Number of cards to place (1-3)

    Returns:
        Image and list of YOLO labels
    """
    # Load random background
    bg_path = random.choice(background_paths)
    background = cv2.imread(str(bg_path))
    if background is None:
        # Create solid color fallback
        color = (random.randint(30, 100), random.randint(30, 100), random.randint(30, 100))
        background = np.full((output_size[1], output_size[0], 3), color, dtype=np.uint8)
    else:
        background = cv2.resize(background, output_size)

    labels = []
    combined_mask = np.zeros((output_size[1], output_size[0]), dtype=np.uint8)

    for _ in range(num_cards):
        # Load random card
        card_path = random.choice(card_paths)
        card_img = cv2.imread(str(card_path), cv2.IMREAD_UNCHANGED)
        if card_img is None:
            continue

        # Convert to BGRA if needed
        if card_img.shape[2] == 3:
            card_img = cv2.cvtColor(card_img, cv2.COLOR_BGR2BGRA)
            card_img[:, :, 3] = 255

        # Apply random transform
        transformed, corners = random_perspective_transform(
            card_img,
            max_rotation=45,
            max_perspective=0.15,
            scale_range=(0.2, 0.6),
        )

        # Apply lighting augmentation to card
        transformed[:, :, :3] = apply_lighting_augmentation(transformed[:, :, :3])

        # Paste onto background
        result, mask, final_corners = paste_card_on_background(transformed, background)
        background = result

        # Add shadow
        background = add_shadow(background, mask)

        # Update combined mask
        combined_mask = np.maximum(combined_mask, mask)

        # Create YOLO label
        label = create_yolo_label(final_corners, class_id=0)
        labels.append(label)

    # Apply final augmentations to whole image
    background = apply_lighting_augmentation(background)

    return background, labels


def main():
    parser = argparse.ArgumentParser(
        description="Generate synthetic dataset for card localization training"
    )
    parser.add_argument(
        "--cards-dir",
        type=str,
        required=True,
        help="Directory containing card images",
    )
    parser.add_argument(
        "--backgrounds-dir",
        type=str,
        default="./data/backgrounds",
        help="Directory containing background images",
    )
    parser.add_argument(
        "--output-dir",
        type=str,
        default="./data/synthetic_dataset",
        help="Output directory for synthetic dataset",
    )
    parser.add_argument(
        "--num-images",
        type=int,
        default=5000,
        help="Number of synthetic images to generate",
    )
    parser.add_argument(
        "--output-size",
        type=int,
        nargs=2,
        default=[640, 640],
        help="Output image size (width height)",
    )
    parser.add_argument(
        "--val-split",
        type=float,
        default=0.1,
        help="Validation split ratio (default: 0.1)",
    )
    parser.add_argument(
        "--download-backgrounds",
        action="store_true",
        help="Download sample background images if none exist",
    )

    args = parser.parse_args()

    cards_dir = Path(args.cards_dir)
    backgrounds_dir = Path(args.backgrounds_dir)
    output_dir = Path(args.output_dir)
    output_size = tuple(args.output_size)

    # Find card images
    card_extensions = {".png", ".jpg", ".jpeg", ".webp"}
    card_paths = [
        p for p in cards_dir.rglob("*")
        if p.suffix.lower() in card_extensions
    ]

    if not card_paths:
        logger.error(f"No card images found in {cards_dir}")
        return 1

    logger.info(f"Found {len(card_paths)} card images")

    # Find or create background images
    if not backgrounds_dir.exists() or args.download_backgrounds:
        logger.info("Generating/downloading background images...")
        backgrounds_dir.mkdir(parents=True, exist_ok=True)
        background_paths = generate_solid_backgrounds(backgrounds_dir, count=15)

        # Try to download some texture backgrounds
        try:
            downloaded = download_sample_backgrounds(backgrounds_dir / "textures", count=5)
            background_paths.extend(downloaded)
        except Exception as e:
            logger.warning(f"Could not download texture backgrounds: {e}")
    else:
        background_paths = list(backgrounds_dir.rglob("*.jpg")) + list(backgrounds_dir.rglob("*.png"))

    if not background_paths:
        logger.info("No backgrounds found, generating solid colors...")
        background_paths = generate_solid_backgrounds(backgrounds_dir, count=10)

    logger.info(f"Using {len(background_paths)} background images")

    # Create output directories
    train_images_dir = output_dir / "images" / "train"
    val_images_dir = output_dir / "images" / "val"
    train_labels_dir = output_dir / "labels" / "train"
    val_labels_dir = output_dir / "labels" / "val"

    for d in [train_images_dir, val_images_dir, train_labels_dir, val_labels_dir]:
        d.mkdir(parents=True, exist_ok=True)

    # Generate synthetic images
    num_val = int(args.num_images * args.val_split)
    num_train = args.num_images - num_val

    logger.info(f"Generating {num_train} training and {num_val} validation images...")

    for i in range(args.num_images):
        is_val = i >= num_train

        # Randomly choose 1-3 cards per image
        num_cards = random.choices([1, 2, 3], weights=[0.7, 0.2, 0.1])[0]

        img, labels = generate_synthetic_image(
            card_paths=card_paths,
            background_paths=background_paths,
            output_size=output_size,
            num_cards=num_cards,
        )

        # Determine output paths
        if is_val:
            img_path = val_images_dir / f"synthetic_{i:06d}.jpg"
            label_path = val_labels_dir / f"synthetic_{i:06d}.txt"
        else:
            img_path = train_images_dir / f"synthetic_{i:06d}.jpg"
            label_path = train_labels_dir / f"synthetic_{i:06d}.txt"

        # Save image and labels
        cv2.imwrite(str(img_path), img)
        with open(label_path, "w") as f:
            f.write("\n".join(labels))

        if (i + 1) % 100 == 0:
            logger.info(f"Generated {i + 1}/{args.num_images} images...")

    # Create dataset YAML for YOLO
    yaml_content = f"""# TCG Card Detection Dataset
path: {output_dir.absolute()}
train: images/train
val: images/val

# Classes
names:
  0: card
"""
    with open(output_dir / "data.yaml", "w") as f:
        f.write(yaml_content)

    logger.info(f"Dataset saved to {output_dir}")
    logger.info(f"  Training images: {num_train}")
    logger.info(f"  Validation images: {num_val}")
    logger.info(f"  YOLO config: {output_dir / 'data.yaml'}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
