import pytest
import numpy as np
import cv2
from unittest.mock import patch, Mock
from src.display.video_processor import VideoFrameProcessor


class TestVideoFrameProcessor:
    """
    Test class for VideoFrameProcessor functionality.
    """

    def setup_method(self):
        """
        Setup method called before each test.
        """
        self.processor = VideoFrameProcessor()

    def test_video_processor_initialization_with_default_values(self):
        """
        Test VideoFrameProcessor initialization with default grid parameters.
        """
        assert self.processor.grid_width == 3
        assert self.processor.grid_height == 2
        assert self.processor.max_cameras == 6
        assert self.processor.frame_width == 640
        assert self.processor.frame_height == 480

    def test_video_processor_initialization_with_custom_values(self):
        """
        Test VideoFrameProcessor initialization with custom grid parameters.
        """
        custom_processor = VideoFrameProcessor(grid_width=4, grid_height=3)
        
        assert custom_processor.grid_width == 4
        assert custom_processor.grid_height == 3
        assert custom_processor.max_cameras == 12

    @patch('cv2.imdecode')
    @patch('numpy.frombuffer')
    def test_decode_frame_successful_decoding(self, mock_frombuffer, mock_imdecode):
        """
        Test successful frame decoding from bytes.
        """
        test_data = b'fake_image_data'
        mock_array = Mock()
        mock_image = np.zeros((480, 640, 3), dtype=np.uint8)
        
        mock_frombuffer.return_value = mock_array
        mock_imdecode.return_value = mock_image
        
        result = self.processor.decode_frame(test_data)
        
        mock_frombuffer.assert_called_once_with(test_data, np.uint8)
        mock_imdecode.assert_called_once_with(mock_array, cv2.IMREAD_COLOR)
        assert np.array_equal(result, mock_image)

    @patch('cv2.imdecode')
    @patch('numpy.frombuffer')
    def test_decode_frame_failed_decoding(self, mock_frombuffer, mock_imdecode):
        """
        Test frame decoding failure returns None.
        """
        test_data = b'invalid_image_data'
        mock_array = Mock()
        
        mock_frombuffer.return_value = mock_array
        mock_imdecode.return_value = None
        
        result = self.processor.decode_frame(test_data)
        
        assert result is None

    @patch('cv2.imdecode')
    @patch('numpy.frombuffer')
    def test_decode_frame_exception_handling(self, mock_frombuffer, mock_imdecode):
        """
        Test frame decoding exception handling.
        """
        test_data = b'corrupt_data'
        mock_frombuffer.side_effect = Exception("Decoding error")
        
        result = self.processor.decode_frame(test_data)
        
        assert result is None

    @patch('cv2.resize')
    def test_resize_frame(self, mock_resize):
        """
        Test frame resizing functionality.
        """
        input_frame = np.zeros((720, 1280, 3), dtype=np.uint8)
        resized_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        
        mock_resize.return_value = resized_frame
        
        result = self.processor.resize_frame(input_frame)
        
        mock_resize.assert_called_once_with(input_frame, (640, 480))
        assert np.array_equal(result, resized_frame)

    @patch('cv2.putText')
    def test_add_camera_label(self, mock_putText):
        """
        Test adding camera label to frame.
        """
        input_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        camera_id = "cam1"
        
        result = self.processor.add_camera_label(input_frame, camera_id)
        
        mock_putText.assert_called_once()
        assert isinstance(result, np.ndarray)

    @patch('cv2.putText')
    def test_create_placeholder_frame(self, mock_putText):
        """
        Test creating placeholder frame for inactive cameras.
        """
        camera_number = 1
        
        result = self.processor.create_placeholder_frame(camera_number)
        
        assert result.shape == (480, 640, 3)
        assert result.dtype == np.uint8
        mock_putText.assert_called_once()

    @patch.object(VideoFrameProcessor, 'add_camera_label')
    @patch.object(VideoFrameProcessor, 'resize_frame')
    @patch.object(VideoFrameProcessor, 'create_placeholder_frame')
    def test_process_frames_for_grid_with_all_cameras(self, mock_placeholder, mock_resize, mock_label):
        """
        Test processing frames when all camera slots are filled.
        """
        camera_frames = {
            'cam1': np.zeros((720, 1280, 3), dtype=np.uint8),
            'cam2': np.zeros((720, 1280, 3), dtype=np.uint8),
            'cam3': np.zeros((720, 1280, 3), dtype=np.uint8),
            'cam4': np.zeros((720, 1280, 3), dtype=np.uint8),
            'cam5': np.zeros((720, 1280, 3), dtype=np.uint8),
            'cam6': np.zeros((720, 1280, 3), dtype=np.uint8)
        }
        
        resized_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        labeled_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        
        mock_resize.return_value = resized_frame
        mock_label.return_value = labeled_frame
        
        result = self.processor.process_frames_for_grid(camera_frames)
        
        assert len(result) == 6
        assert mock_resize.call_count == 6
        assert mock_label.call_count == 6
        assert not mock_placeholder.called

    @patch.object(VideoFrameProcessor, 'add_camera_label')
    @patch.object(VideoFrameProcessor, 'resize_frame')
    @patch.object(VideoFrameProcessor, 'create_placeholder_frame')
    def test_process_frames_for_grid_with_partial_cameras(self, mock_placeholder, mock_resize, mock_label):
        """
        Test processing frames when only some camera slots are filled.
        """
        camera_frames = {
            'cam1': np.zeros((720, 1280, 3), dtype=np.uint8),
            'cam2': np.zeros((720, 1280, 3), dtype=np.uint8)
        }
        
        resized_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        labeled_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        placeholder_frame = np.zeros((480, 640, 3), dtype=np.uint8)
        
        mock_resize.return_value = resized_frame
        mock_label.return_value = labeled_frame
        mock_placeholder.return_value = placeholder_frame
        
        result = self.processor.process_frames_for_grid(camera_frames)
        
        assert len(result) == 6
        assert mock_resize.call_count == 2
        assert mock_label.call_count == 2
        assert mock_placeholder.call_count == 4

    @patch('numpy.vstack')
    @patch('numpy.hstack')
    def test_create_grid_display_success(self, mock_hstack, mock_vstack):
        """
        Test successful grid display creation.
        """
        processed_frames = [np.zeros((480, 640, 3), dtype=np.uint8) for _ in range(6)]
        row_frame = np.zeros((480, 1920, 3), dtype=np.uint8)
        grid_frame = np.zeros((960, 1920, 3), dtype=np.uint8)
        
        mock_hstack.return_value = row_frame
        mock_vstack.return_value = grid_frame
        
        result = self.processor.create_grid_display(processed_frames)
        
        assert mock_hstack.call_count == 2
        mock_vstack.assert_called_once()
        assert np.array_equal(result, grid_frame)

    def test_create_grid_display_invalid_frame_count(self):
        """
        Test grid display creation with invalid frame count raises ValueError.
        """
        invalid_frames = [np.zeros((480, 640, 3), dtype=np.uint8) for _ in range(4)]
        
        with pytest.raises(ValueError, match="Expected 6 frames, got 4"):
            self.processor.create_grid_display(invalid_frames)