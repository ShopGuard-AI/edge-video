import time
from typing import Optional
from .config.config_manager import ConfigManager, RabbitMQConfig
from .consumer.rabbitmq_consumer import RabbitMQConsumer  
from .display.video_processor import VideoFrameProcessor
from .display.display_manager import VideoDisplayManager


class VideoConsumerApplication:
    """
    Main application class that orchestrates video consumption and display.
    """

    def __init__(self, config_manager: ConfigManager) -> None:
        """
        Initializes the video consumer application.

        Args:
            config_manager (ConfigManager): Configuration manager instance.
        """
        self.config_manager = config_manager
        self.rabbitmq_config = config_manager.get_rabbitmq_config()
        
        self.consumer = RabbitMQConsumer(self.rabbitmq_config)
        self.video_processor = VideoFrameProcessor()
        self.display_manager = VideoDisplayManager()
        
        self.consumer.set_message_callback(self._handle_frame_message)
        self.is_running = False

    def _handle_frame_message(self, camera_id: str, frame_data: bytes) -> None:
        """
        Handles incoming frame messages from RabbitMQ.

        Args:
            camera_id (str): Identifier of the camera.
            frame_data (bytes): Raw frame data.
        """
        decoded_frame = self.video_processor.decode_frame(frame_data)
        if decoded_frame is not None:
            self.display_manager.update_camera_frame(camera_id, decoded_frame)
        else:
            print(f"Failed to decode frame from camera '{camera_id}'.")

    def start(self) -> bool:
        """
        Starts the video consumer application.

        Returns:
            bool: True if started successfully, False otherwise.
        """
        if not self.consumer.connect():
            print("Failed to connect to RabbitMQ. Check configuration and server status.")
            return False
        
        self.consumer.start_consuming()
        self.is_running = True
        print("[INFO] Press 'q' in any window to exit.")
        
        return True

    def run_main_loop(self) -> None:
        """
        Runs the main application loop for processing and display.
        """
        try:
            while self.is_running:
                self.consumer.process_data_events(time_limit_seconds=0.1)
                
                if self.display_manager.has_camera_frames():
                    self._update_display()
                
                if self.display_manager.check_exit_key():
                    print("Exiting by user command.")
                    self.stop()
                    break
                    
        except KeyboardInterrupt:
            self.consumer.handle_keyboard_interrupt()
            self.stop()

    def _update_display(self) -> None:
        """
        Updates the video display with current frames.
        """
        camera_frames = self.display_manager.get_camera_frames()
        processed_frames = self.video_processor.process_frames_for_grid(camera_frames)
        grid_display = self.video_processor.create_grid_display(processed_frames)
        self.display_manager.display_grid(grid_display)

    def stop(self) -> None:
        """
        Stops the application and cleanup resources.
        """
        self.is_running = False
        self.display_manager.close_windows()
        self.consumer.disconnect()

    def get_running_status(self) -> bool:
        """
        Returns the current running status.

        Returns:
            bool: True if running, False otherwise.
        """
        return self.is_running