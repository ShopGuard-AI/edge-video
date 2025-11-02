import cv2
import numpy as np
from typing import Dict, List, Optional


class VideoFrameProcessor:
    """
    A class to handle video frame processing operations.
    """

    def __init__(self, grid_width: int = 3, grid_height: int = 2) -> None:
        """
        Initializes the video frame processor.

        Args:
            grid_width (int): Number of cameras per row in the grid.
            grid_height (int): Number of rows in the grid.
        """
        self.grid_width = grid_width
        self.grid_height = grid_height
        self.max_cameras = grid_width * grid_height
        self.frame_width = 640
        self.frame_height = 480

    def decode_frame(self, frame_data: bytes) -> Optional[np.ndarray]:
        """
        Decodes frame data from bytes to OpenCV image format.

        Args:
            frame_data (bytes): Raw frame data in JPEG/PNG format.

        Returns:
            Optional[np.ndarray]: Decoded image or None if decoding fails.
        """
        try:
            numpy_array = np.frombuffer(frame_data, np.uint8)
            image = cv2.imdecode(numpy_array, cv2.IMREAD_COLOR)
            return image
        except Exception as exception_error:
            print(f"Error decoding frame: {exception_error}")
            return None

    def resize_frame(self, frame: np.ndarray) -> np.ndarray:
        """
        Resizes frame to standard dimensions for grid display.

        Args:
            frame (np.ndarray): Input frame to resize.

        Returns:
            np.ndarray: Resized frame.
        """
        return cv2.resize(frame, (self.frame_width, self.frame_height))

    def add_camera_label(self, frame: np.ndarray, camera_id: str) -> np.ndarray:
        """
        Adds camera ID label to the frame.

        Args:
            frame (np.ndarray): Input frame to label.
            camera_id (str): Camera identifier to display.

        Returns:
            np.ndarray: Frame with added label.
        """
        labeled_frame = frame.copy()
        cv2.putText(
            labeled_frame, 
            camera_id, 
            (10, 30), 
            cv2.FONT_HERSHEY_SIMPLEX,
            1, 
            (0, 255, 0), 
            2, 
            cv2.LINE_AA
        )
        return labeled_frame

    def create_placeholder_frame(self, camera_number: int) -> np.ndarray:
        """
        Creates a black placeholder frame for inactive cameras.

        Args:
            camera_number (int): Camera number to display on placeholder.

        Returns:
            np.ndarray: Black frame with camera number.
        """
        placeholder_frame = np.zeros((self.frame_height, self.frame_width, 3), dtype=np.uint8)
        camera_text = f"Cam{camera_number}"
        cv2.putText(
            placeholder_frame, 
            camera_text, 
            (250, 240), 
            cv2.FONT_HERSHEY_SIMPLEX,
            1, 
            (255, 255, 255), 
            2, 
            cv2.LINE_AA
        )
        return placeholder_frame

    def process_frames_for_grid(self, camera_frames: Dict[str, np.ndarray]) -> List[np.ndarray]:
        """
        Processes camera frames for grid display.

        Args:
            camera_frames (Dict[str, np.ndarray]): Dictionary of camera frames.

        Returns:
            List[np.ndarray]: List of processed frames ready for grid display.
        """
        processed_frames = []
        camera_ids = sorted(camera_frames.keys())
        
        for camera_index in range(self.max_cameras):
            if camera_index < len(camera_ids):
                camera_id = camera_ids[camera_index]
                frame = camera_frames[camera_id]
                resized_frame = self.resize_frame(frame)
                labeled_frame = self.add_camera_label(resized_frame, camera_id)
                processed_frames.append(labeled_frame)
            else:
                placeholder_frame = self.create_placeholder_frame(camera_index + 1)
                processed_frames.append(placeholder_frame)
        
        return processed_frames

    def create_grid_display(self, processed_frames: List[np.ndarray]) -> np.ndarray:
        """
        Creates a grid display from processed frames.

        Args:
            processed_frames (List[np.ndarray]): List of processed frames.

        Returns:
            np.ndarray: Combined grid display.
        """
        if len(processed_frames) != self.max_cameras:
            raise ValueError(f"Expected {self.max_cameras} frames, got {len(processed_frames)}")
        
        grid_rows = []
        for row_index in range(self.grid_height):
            start_index = row_index * self.grid_width
            end_index = start_index + self.grid_width
            row_frames = processed_frames[start_index:end_index]
            grid_row = np.hstack(row_frames)
            grid_rows.append(grid_row)
        
        return np.vstack(grid_rows)