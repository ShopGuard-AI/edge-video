from src.config.config_manager import ConfigManager
from src.video_consumer_app import VideoConsumerApplication


def main() -> None:
    """
    Main entry point for the video consumer application.
    """
    config_manager = ConfigManager()
    video_app = VideoConsumerApplication(config_manager)
    
    if video_app.start():
        video_app.run_main_loop()
    else:
        print("Failed to start video consumer application.")


if __name__ == '__main__':
    main()