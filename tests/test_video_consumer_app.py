import pytest
from unittest.mock import patch, Mock, MagicMock
import numpy as np
from src.video_consumer_app import VideoConsumerApplication
from src.config.config_manager import ConfigManager


class TestVideoConsumerApplication:
    """
    Test class for VideoConsumerApplication functionality.
    """

    def setup_method(self):
        """
        Setup method called before each test.
        """
        self.config_manager = ConfigManager()
        self.app = VideoConsumerApplication(self.config_manager)

    @patch('src.video_consumer_app.RabbitMQConsumer')
    @patch('src.video_consumer_app.VideoFrameProcessor')
    @patch('src.video_consumer_app.VideoDisplayManager')
    def test_video_consumer_app_initialization(self, mock_display, mock_processor, mock_consumer):
        """
        Test VideoConsumerApplication initialization.
        """
        config_manager = ConfigManager()
        app = VideoConsumerApplication(config_manager)
        
        assert app.config_manager == config_manager
        assert app.is_running is False
        mock_consumer.assert_called_once()
        mock_processor.assert_called_once()
        mock_display.assert_called_once()

    def test_handle_frame_message_successful_decoding(self):
        """
        Test _handle_frame_message with successful frame decoding.
        """
        test_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        self.app.video_processor.decode_frame = Mock(return_value=test_frame)
        self.app.display_manager.update_camera_frame = Mock()
        
        self.app._handle_frame_message("cam1", b"frame_data")
        
        self.app.video_processor.decode_frame.assert_called_once_with(b"frame_data")
        self.app.display_manager.update_camera_frame.assert_called_once_with("cam1", test_frame)

    def test_handle_frame_message_failed_decoding(self):
        """
        Test _handle_frame_message with failed frame decoding.
        """
        self.app.video_processor.decode_frame = Mock(return_value=None)
        self.app.display_manager.update_camera_frame = Mock()
        
        self.app._handle_frame_message("cam1", b"invalid_data")
        
        self.app.video_processor.decode_frame.assert_called_once_with(b"invalid_data")
        self.app.display_manager.update_camera_frame.assert_not_called()

    def test_start_successful_connection(self):
        """
        Test start method with successful RabbitMQ connection.
        """
        self.app.consumer.connect = Mock(return_value=True)
        self.app.consumer.start_consuming = Mock()
        
        result = self.app.start()
        
        assert result is True
        assert self.app.is_running is True
        self.app.consumer.connect.assert_called_once()
        self.app.consumer.start_consuming.assert_called_once()

    def test_start_failed_connection(self):
        """
        Test start method with failed RabbitMQ connection.
        """
        self.app.consumer.connect = Mock(return_value=False)
        self.app.consumer.start_consuming = Mock()
        
        result = self.app.start()
        
        assert result is False
        assert self.app.is_running is False
        self.app.consumer.connect.assert_called_once()
        self.app.consumer.start_consuming.assert_not_called()

    def test_update_display(self):
        """
        Test _update_display method functionality.
        """
        test_frames = {"cam1": np.zeros((480, 640, 3), dtype=np.uint8)}
        processed_frames = [np.zeros((480, 640, 3), dtype=np.uint8) for _ in range(6)]
        grid_display = np.zeros((960, 1920, 3), dtype=np.uint8)
        
        self.app.display_manager.get_camera_frames = Mock(return_value=test_frames)
        self.app.video_processor.process_frames_for_grid = Mock(return_value=processed_frames)
        self.app.video_processor.create_grid_display = Mock(return_value=grid_display)
        self.app.display_manager.display_grid = Mock()
        
        self.app._update_display()
        
        self.app.display_manager.get_camera_frames.assert_called_once()
        self.app.video_processor.process_frames_for_grid.assert_called_once_with(test_frames)
        self.app.video_processor.create_grid_display.assert_called_once_with(processed_frames)
        self.app.display_manager.display_grid.assert_called_once_with(grid_display)

    def test_run_main_loop_normal_exit(self):
        """
        Test run_main_loop with normal exit condition.
        """
        self.app.is_running = True
        self.app.consumer.process_data_events = Mock()
        self.app.display_manager.has_camera_frames = Mock(return_value=False)
        self.app.display_manager.check_exit_key = Mock(side_effect=[False, True])
        self.app.stop = Mock()
        
        self.app.run_main_loop()
        
        assert self.app.consumer.process_data_events.call_count >= 1
        self.app.stop.assert_called_once()

    def test_run_main_loop_keyboard_interrupt(self):
        """
        Test run_main_loop with keyboard interrupt.
        """
        self.app.is_running = True
        self.app.consumer.process_data_events = Mock(side_effect=KeyboardInterrupt)
        self.app.consumer.handle_keyboard_interrupt = Mock()
        self.app.stop = Mock()
        
        self.app.run_main_loop()
        
        self.app.consumer.handle_keyboard_interrupt.assert_called_once()
        self.app.stop.assert_called_once()

    def test_run_main_loop_with_display_update(self):
        """
        Test run_main_loop with display update.
        """
        self.app.is_running = True
        self.app.consumer.process_data_events = Mock()
        self.app.display_manager.has_camera_frames = Mock(side_effect=[True, False])
        self.app.display_manager.check_exit_key = Mock(side_effect=[False, True])
        self.app._update_display = Mock()
        self.app.stop = Mock()
        
        self.app.run_main_loop()
        
        self.app._update_display.assert_called_once()

    def test_stop(self):
        """
        Test stop method functionality.
        """
        self.app.is_running = True
        self.app.display_manager.close_windows = Mock()
        self.app.consumer.disconnect = Mock()
        
        self.app.stop()
        
        assert self.app.is_running is False
        self.app.display_manager.close_windows.assert_called_once()
        self.app.consumer.disconnect.assert_called_once()

    def test_get_running_status_when_running(self):
        """
        Test get_running_status returns True when running.
        """
        self.app.is_running = True
        
        result = self.app.get_running_status()
        
        assert result is True

    def test_get_running_status_when_not_running(self):
        """
        Test get_running_status returns False when not running.
        """
        self.app.is_running = False
        
        result = self.app.get_running_status()
        
        assert result is False