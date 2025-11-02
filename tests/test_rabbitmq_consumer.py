import pytest
from unittest.mock import patch, Mock, MagicMock
from src.consumer.rabbitmq_consumer import RabbitMQConsumer
from src.config.config_manager import RabbitMQConfig


class TestRabbitMQConsumer:
    """
    Test class for RabbitMQConsumer functionality.
    """

    def setup_method(self):
        """
        Setup method called before each test.
        """
        self.config = RabbitMQConfig(
            host='localhost',
            port=5672,
            virtual_host='test_vhost',
            username='test_user',
            password='test_pass',
            exchange_name='test_exchange',
            routing_key='camera.#',
            queue_name='test_queue'
        )
        self.consumer = RabbitMQConsumer(self.config)

    def test_rabbitmq_consumer_initialization(self):
        """
        Test RabbitMQConsumer initialization.
        """
        assert self.consumer.config == self.config
        assert self.consumer.connection is None
        assert self.consumer.channel is None
        assert self.consumer.queue_name is None
        assert self.consumer.message_callback is None

    @patch('pika.BlockingConnection')
    @patch('pika.PlainCredentials')
    @patch('pika.ConnectionParameters')
    def test_connect_successful(self, mock_params, mock_credentials, mock_connection):
        """
        Test successful connection to RabbitMQ.
        """
        mock_conn = Mock()
        mock_channel = Mock()
        mock_queue_result = Mock()
        mock_queue_result.method.queue = 'test_queue_name'
        
        mock_connection.return_value = mock_conn
        mock_conn.channel.return_value = mock_channel
        mock_channel.queue_declare.return_value = mock_queue_result
        
        result = self.consumer.connect()
        
        assert result is True
        mock_credentials.assert_called_once_with('test_user', 'test_pass')
        mock_connection.assert_called_once()
        mock_channel.exchange_declare.assert_called_once_with(
            exchange='test_exchange',
            exchange_type='topic',
            durable=True
        )
        mock_channel.queue_bind.assert_called_once()

    @patch('pika.BlockingConnection')
    def test_connect_amqp_connection_error(self, mock_connection):
        """
        Test connection failure with AMQPConnectionError.
        """
        import pika
        mock_connection.side_effect = pika.exceptions.AMQPConnectionError("Connection failed")
        
        result = self.consumer.connect()
        
        assert result is False

    @patch('pika.BlockingConnection')
    def test_connect_general_exception(self, mock_connection):
        """
        Test connection failure with general exception.
        """
        mock_connection.side_effect = Exception("Unexpected error")
        
        result = self.consumer.connect()
        
        assert result is False

    def test_set_message_callback(self):
        """
        Test setting message callback function.
        """
        callback_function = Mock()
        
        self.consumer.set_message_callback(callback_function)
        
        assert self.consumer.message_callback == callback_function

    def test_start_consuming_without_proper_setup_raises_error(self):
        """
        Test start_consuming raises error when not properly configured.
        """
        with pytest.raises(RuntimeError, match="Consumer not properly configured"):
            self.consumer.start_consuming()

    def test_start_consuming_with_proper_setup(self):
        """
        Test start_consuming with proper configuration.
        """
        mock_channel = Mock()
        callback_function = Mock()
        
        self.consumer.channel = mock_channel
        self.consumer.queue_name = 'test_queue'
        self.consumer.message_callback = callback_function
        
        self.consumer.start_consuming()
        
        mock_channel.basic_consume.assert_called_once_with(
            queue='test_queue',
            on_message_callback=self.consumer._on_message_received,
            auto_ack=True
        )

    def test_on_message_received_calls_callback(self):
        """
        Test _on_message_received method calls message callback.
        """
        callback_function = Mock()
        mock_method = Mock()
        mock_method.routing_key = 'camera.cam1'
        
        self.consumer.message_callback = callback_function
        
        self.consumer._on_message_received(None, mock_method, None, b'frame_data')
        
        callback_function.assert_called_once_with('cam1', b'frame_data')

    def test_process_data_events_with_connection(self):
        """
        Test process_data_events with active connection.
        """
        mock_connection = Mock()
        self.consumer.connection = mock_connection
        
        self.consumer.process_data_events(0.5)
        
        mock_connection.process_data_events.assert_called_once_with(time_limit=0.5)

    def test_process_data_events_without_connection(self):
        """
        Test process_data_events without connection does nothing.
        """
        self.consumer.connection = None
        
        # Should not raise any exception
        self.consumer.process_data_events()

    def test_is_connected_with_active_connection(self):
        """
        Test is_connected returns True with active connection.
        """
        mock_connection = Mock()
        mock_connection.is_closed = False
        self.consumer.connection = mock_connection
        
        result = self.consumer.is_connected()
        
        assert result is True

    def test_is_connected_with_closed_connection(self):
        """
        Test is_connected returns False with closed connection.
        """
        mock_connection = Mock()
        mock_connection.is_closed = True
        self.consumer.connection = mock_connection
        
        result = self.consumer.is_connected()
        
        assert result is False

    def test_is_connected_without_connection(self):
        """
        Test is_connected returns False without connection.
        """
        self.consumer.connection = None
        
        result = self.consumer.is_connected()
        
        assert result is False

    def test_disconnect_with_active_connection(self):
        """
        Test disconnect with active connection.
        """
        mock_connection = Mock()
        mock_connection.is_closed = False
        self.consumer.connection = mock_connection
        
        self.consumer.disconnect()
        
        mock_connection.close.assert_called_once()
        assert self.consumer.connection is None
        assert self.consumer.channel is None
        assert self.consumer.queue_name is None

    def test_disconnect_with_exception(self):
        """
        Test disconnect handles exception gracefully.
        """
        mock_connection = Mock()
        mock_connection.is_closed = False
        mock_connection.close.side_effect = Exception("Close error")
        self.consumer.connection = mock_connection
        
        # Should not raise exception
        self.consumer.disconnect()
        
        assert self.consumer.connection is None

    @patch('sys.exit')
    @patch('os._exit')
    def test_handle_keyboard_interrupt(self, mock_os_exit, mock_sys_exit):
        """
        Test handle_keyboard_interrupt method.
        """
        mock_connection = Mock()
        self.consumer.connection = mock_connection
        
        mock_sys_exit.side_effect = SystemExit()
        
        self.consumer.handle_keyboard_interrupt()
        
        mock_connection.close.assert_called_once()
        mock_sys_exit.assert_called_once_with(0)
        mock_os_exit.assert_called_once_with(0)