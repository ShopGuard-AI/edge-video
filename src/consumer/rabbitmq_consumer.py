import pika
import sys
from typing import Callable, Optional
from ..config.config_manager import RabbitMQConfig


class RabbitMQConsumer:
    """
    A class to handle RabbitMQ connection and message consumption.
    """

    def __init__(self, config: RabbitMQConfig) -> None:
        """
        Initializes the RabbitMQ consumer with configuration.

        Args:
            config (RabbitMQConfig): RabbitMQ connection configuration.
        """
        self.config = config
        self.connection: Optional[pika.BlockingConnection] = None
        self.channel: Optional[pika.channel.Channel] = None
        self.queue_name: Optional[str] = None
        self.message_callback: Optional[Callable] = None

    def connect(self) -> bool:
        """
        Establishes connection to RabbitMQ server.

        Returns:
            bool: True if connection successful, False otherwise.
        """
        try:
            credentials = pika.PlainCredentials(self.config.username, self.config.password)
            connection_parameters = pika.ConnectionParameters(
                host=self.config.host,
                port=self.config.port,
                virtual_host=self.config.virtual_host,
                credentials=credentials
            )
            
            self.connection = pika.BlockingConnection(connection_parameters)
            self.channel = self.connection.channel()
            
            self.channel.exchange_declare(
                exchange=self.config.exchange_name, 
                exchange_type='topic', 
                durable=True
            )
            
            queue_result = self.channel.queue_declare(
                queue=self.config.queue_name, 
                exclusive=True, 
                durable=False
            )
            self.queue_name = queue_result.method.queue
            
            self.channel.queue_bind(
                exchange=self.config.exchange_name,
                queue=self.queue_name,
                routing_key=self.config.routing_key
            )
            
            return True
            
        except pika.exceptions.AMQPConnectionError as connection_error:
            print(f"RabbitMQ connection error: {connection_error}")
            return False
        except Exception as general_error:
            print(f"Unexpected error during connection: {general_error}")
            return False

    def set_message_callback(self, callback: Callable) -> None:
        """
        Sets the callback function for message processing.

        Args:
            callback (Callable): Function to call when message is received.
        """
        self.message_callback = callback

    def start_consuming(self) -> None:
        """
        Starts consuming messages from the queue.
        """
        if not self.channel or not self.queue_name or not self.message_callback:
            raise RuntimeError("Consumer not properly configured. Ensure connection, queue, and callback are set.")
        
        self.channel.basic_consume(
            queue=self.queue_name,
            on_message_callback=self._on_message_received,
            auto_ack=True
        )
        
        print(f"[*] Queue '{self.queue_name}' created. Waiting for frames...")

    def _on_message_received(self, channel, method, properties, body) -> None:
        """
        Internal callback method for processing received messages.

        Args:
            channel: RabbitMQ channel object.
            method: Method frame with routing key information.
            properties: Message properties.
            body: Message body content.
        """
        camera_id = method.routing_key.replace('camera.', '')
        print(f" [x] Received frame from camera '{camera_id}'. Size: {len(body)} bytes")
        
        if self.message_callback:
            self.message_callback(camera_id, body)

    def process_data_events(self, time_limit_seconds: float = 0.1) -> None:
        """
        Processes data events for non-blocking consumption.

        Args:
            time_limit_seconds (float): Time limit for processing events.
        """
        if self.connection:
            self.connection.process_data_events(time_limit=time_limit_seconds)

    def is_connected(self) -> bool:
        """
        Checks if the connection is active.

        Returns:
            bool: True if connected, False otherwise.
        """
        return self.connection is not None and not self.connection.is_closed

    def disconnect(self) -> None:
        """
        Closes the RabbitMQ connection and cleanup resources.
        """
        try:
            if self.connection and not self.connection.is_closed:
                self.connection.close()
        except Exception as close_error:
            print(f"Error closing connection: {close_error}")
        finally:
            self.connection = None
            self.channel = None
            self.queue_name = None

    def handle_keyboard_interrupt(self) -> None:
        """
        Handles keyboard interrupt gracefully.
        """
        print('Interrupted by user. Shutting down.')
        self.disconnect()
        try:
            sys.exit(0)
        except SystemExit:
            import os
            os._exit(0)