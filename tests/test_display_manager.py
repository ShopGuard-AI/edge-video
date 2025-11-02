import pytest
import numpy as np
from unittest.mock import patch, Mock
from src.display.display_manager import VideoDisplayManager


class TestVideoDisplayManager:
    """
    Test class for VideoDisplayManager functionality.
    """

    def setup_method(self):
        """
        Setup method called before each test.
        """
        self.display_manager = VideoDisplayManager()

    def test_video_display_manager_initialization_with_default_title(self):
        """
        Test VideoDisplayManager initialization with default window title.
        """
        assert self.display_manager.window_title == "Cameras Grid (2x3)"
        assert self.display_manager.is_window_open is False
        assert len(self.display_manager.camera_frames) == 0

    def test_video_display_manager_initialization_with_custom_title(self):
        """
        Test VideoDisplayManager initialization with custom window title.
        """
        custom_manager = VideoDisplayManager("Custom Window Title")
        
        assert custom_manager.window_title == "Custom Window Title"
        assert custom_manager.is_window_open is False

    def test_update_camera_frame_with_valid_frame(self):
        """
        Test updating camera frame with valid frame data.
        """
        camera_id = "cam1"
        test_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        
        self.display_manager.update_camera_frame(camera_id, test_frame)
        
        assert camera_id in self.display_manager.camera_frames
        assert np.array_equal(self.display_manager.camera_frames[camera_id], test_frame)

    def test_update_camera_frame_with_none_frame(self):
        """
        Test updating camera frame with None frame data doesn't store it.
        """
        camera_id = "cam1"
        initial_count = len(self.display_manager.camera_frames)
        
        self.display_manager.update_camera_frame(camera_id, None)
        
        assert len(self.display_manager.camera_frames) == initial_count
        assert camera_id not in self.display_manager.camera_frames

    @patch('cv2.imshow')
    def test_display_grid(self, mock_imshow):
        """
        Test displaying grid image.
        """
        grid_image = np.zeros((960, 1920, 3), dtype=np.uint8)
        
        self.display_manager.display_grid(grid_image)
        
        mock_imshow.assert_called_once_with(self.display_manager.window_title, grid_image)
        assert self.display_manager.is_window_open is True

    @patch('cv2.waitKey')
    def test_check_exit_key_with_q_pressed(self, mock_waitKey):
        """
        Test check_exit_key returns True when 'q' is pressed.
        """
        mock_waitKey.return_value = ord('q')
        
        result = self.display_manager.check_exit_key()
        
        mock_waitKey.assert_called_once_with(1)
        assert result is True

    @patch('cv2.waitKey')
    def test_check_exit_key_with_other_key_pressed(self, mock_waitKey):
        """
        Test check_exit_key returns False when other key is pressed.
        """
        mock_waitKey.return_value = ord('a')
        
        result = self.display_manager.check_exit_key()
        
        mock_waitKey.assert_called_once_with(1)
        assert result is False

    @patch('cv2.waitKey')
    def test_check_exit_key_with_custom_wait_time(self, mock_waitKey):
        """
        Test check_exit_key with custom wait time.
        """
        mock_waitKey.return_value = ord('a')
        
        self.display_manager.check_exit_key(wait_time_ms=5)
        
        mock_waitKey.assert_called_once_with(5)

    def test_has_camera_frames_when_empty(self):
        """
        Test has_camera_frames returns False when no frames exist.
        """
        result = self.display_manager.has_camera_frames()
        
        assert result is False

    def test_has_camera_frames_when_frames_exist(self):
        """
        Test has_camera_frames returns True when frames exist.
        """
        test_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        self.display_manager.update_camera_frame("cam1", test_frame)
        
        result = self.display_manager.has_camera_frames()
        
        assert result is True

    def test_get_camera_frames_returns_copy(self):
        """
        Test get_camera_frames returns a copy of camera frames.
        """
        test_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        self.display_manager.update_camera_frame("cam1", test_frame)
        
        frames_copy = self.display_manager.get_camera_frames()
        
        assert frames_copy is not self.display_manager.camera_frames
        assert "cam1" in frames_copy
        assert np.array_equal(frames_copy["cam1"], test_frame)

    def test_clear_camera_frames(self):
        """
        Test clearing all camera frames.
        """
        test_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        self.display_manager.update_camera_frame("cam1", test_frame)
        
        self.display_manager.clear_camera_frames()
        
        assert len(self.display_manager.camera_frames) == 0

    @patch('cv2.destroyAllWindows')
    def test_close_windows(self, mock_destroyAllWindows):
        """
        Test closing windows and cleanup.
        """
        self.display_manager.is_window_open = True
        
        self.display_manager.close_windows()
        
        mock_destroyAllWindows.assert_called_once()
        assert self.display_manager.is_window_open is False

    def test_get_window_status_when_closed(self):
        """
        Test get_window_status returns False when window is closed.
        """
        result = self.display_manager.get_window_status()
        
        assert result is False

    def test_get_window_status_when_open(self):
        """
        Test get_window_status returns True when window is open.
        """
        self.display_manager.is_window_open = True
        
        result = self.display_manager.get_window_status()
        
        assert result is True