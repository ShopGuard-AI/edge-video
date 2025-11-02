import cv2
import numpy as np
from typing import Dict


class VideoDisplayManager:
    """
    A class to manage video display operations using OpenCV.
    """

    def __init__(self, window_title: str = "Cameras Grid (2x3)") -> None:
        """
        Initializes the video display manager.

        Args:
            window_title (str): Title for the display window.
        """
        self.window_title = window_title
        self.is_window_open = False
        self.camera_frames: Dict[str, np.ndarray] = {}

    def update_camera_frame(self, camera_id: str, frame: np.ndarray) -> None:
        """
        Updates the frame for a specific camera.

        Args:
            camera_id (str): Identifier for the camera.
            frame (np.ndarray): New frame data.
        """
        if frame is not None:
            self.camera_frames[camera_id] = frame

    def display_grid(self, grid_image: np.ndarray) -> None:
        """
        Displays the grid image in the OpenCV window.

        Args:
            grid_image (np.ndarray): Grid image to display.
        """
        cv2.imshow(self.window_title, grid_image)
        if not self.is_window_open:
            self.is_window_open = True

    def check_exit_key(self, wait_time_ms: int = 1) -> bool:
        """
        Checks if the exit key ('q') has been pressed.

        Args:
            wait_time_ms (int): Time to wait for key press in milliseconds.

        Returns:
            bool: True if exit key was pressed, False otherwise.
        """
        key_pressed = cv2.waitKey(wait_time_ms) & 0xFF
        return key_pressed == ord('q')

    def has_camera_frames(self) -> bool:
        """
        Checks if there are any camera frames available.

        Returns:
            bool: True if camera frames exist, False otherwise.
        """
        return len(self.camera_frames) > 0

    def get_camera_frames(self) -> Dict[str, np.ndarray]:
        """
        Returns the current camera frames.

        Returns:
            Dict[str, np.ndarray]: Dictionary of camera frames.
        """
        return self.camera_frames.copy()

    def clear_camera_frames(self) -> None:
        """
        Clears all stored camera frames.
        """
        self.camera_frames.clear()

    def close_windows(self) -> None:
        """
        Closes all OpenCV windows and cleanup resources.
        """
        cv2.destroyAllWindows()
        self.is_window_open = False

    def get_window_status(self) -> bool:
        """
        Returns the current window open status.

        Returns:
            bool: True if window is open, False otherwise.
        """
        return self.is_window_open