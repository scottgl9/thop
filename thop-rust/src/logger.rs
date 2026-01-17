//! Simple logging module for thop

use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::PathBuf;
use std::sync::Mutex;

/// Log levels
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum LogLevel {
    Off,
    Error,
    Warn,
    Info,
    Debug,
}

impl LogLevel {
    pub fn from_str(s: &str) -> Self {
        match s.to_lowercase().as_str() {
            "off" | "none" => LogLevel::Off,
            "error" => LogLevel::Error,
            "warn" | "warning" => LogLevel::Warn,
            "info" => LogLevel::Info,
            "debug" => LogLevel::Debug,
            _ => LogLevel::Info,
        }
    }
}

/// Global logger state
static LOGGER: Mutex<Option<Logger>> = Mutex::new(None);

/// Logger configuration and state
pub struct Logger {
    level: LogLevel,
    log_file: Option<PathBuf>,
}

impl Logger {
    /// Initialize the global logger
    pub fn init(level: LogLevel, log_file: Option<PathBuf>) {
        let mut logger = LOGGER.lock().unwrap();
        *logger = Some(Logger { level, log_file });
    }

    /// Get the default log file path
    pub fn default_log_path() -> PathBuf {
        dirs::data_dir()
            .unwrap_or_else(|| dirs::home_dir().unwrap_or_else(|| PathBuf::from(".")))
            .join("thop")
            .join("thop.log")
    }

    /// Log a message at the specified level
    fn log(&self, level: LogLevel, message: &str) {
        if level > self.level {
            return;
        }

        let level_str = match level {
            LogLevel::Off => return,
            LogLevel::Error => "ERROR",
            LogLevel::Warn => "WARN",
            LogLevel::Info => "INFO",
            LogLevel::Debug => "DEBUG",
        };

        let timestamp = chrono::Local::now().format("%Y-%m-%d %H:%M:%S");
        let formatted = format!("[{}] {} - {}\n", timestamp, level_str, message);

        // Write to log file if configured
        if let Some(ref path) = self.log_file {
            if let Some(parent) = path.parent() {
                fs::create_dir_all(parent).ok();
            }

            if let Ok(mut file) = OpenOptions::new()
                .create(true)
                .append(true)
                .open(path)
            {
                file.write_all(formatted.as_bytes()).ok();
            }
        }

        // Also write to stderr for error level in debug mode
        if level == LogLevel::Error || (level == LogLevel::Debug && self.level >= LogLevel::Debug) {
            eprint!("{}", formatted);
        }
    }
}

/// Log an error message
pub fn error(message: &str) {
    if let Ok(guard) = LOGGER.lock() {
        if let Some(ref logger) = *guard {
            logger.log(LogLevel::Error, message);
        }
    }
}

/// Log a warning message
pub fn warn(message: &str) {
    if let Ok(guard) = LOGGER.lock() {
        if let Some(ref logger) = *guard {
            logger.log(LogLevel::Warn, message);
        }
    }
}

/// Log an info message
pub fn info(message: &str) {
    if let Ok(guard) = LOGGER.lock() {
        if let Some(ref logger) = *guard {
            logger.log(LogLevel::Info, message);
        }
    }
}

/// Log a debug message
pub fn debug(message: &str) {
    if let Ok(guard) = LOGGER.lock() {
        if let Some(ref logger) = *guard {
            logger.log(LogLevel::Debug, message);
        }
    }
}

/// Log a formatted error message
#[macro_export]
macro_rules! log_error {
    ($($arg:tt)*) => {
        $crate::logger::error(&format!($($arg)*))
    };
}

/// Log a formatted warning message
#[macro_export]
macro_rules! log_warn {
    ($($arg:tt)*) => {
        $crate::logger::warn(&format!($($arg)*))
    };
}

/// Log a formatted info message
#[macro_export]
macro_rules! log_info {
    ($($arg:tt)*) => {
        $crate::logger::info(&format!($($arg)*))
    };
}

/// Log a formatted debug message
#[macro_export]
macro_rules! log_debug {
    ($($arg:tt)*) => {
        $crate::logger::debug(&format!($($arg)*))
    };
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_log_level_from_str() {
        assert_eq!(LogLevel::from_str("debug"), LogLevel::Debug);
        assert_eq!(LogLevel::from_str("DEBUG"), LogLevel::Debug);
        assert_eq!(LogLevel::from_str("info"), LogLevel::Info);
        assert_eq!(LogLevel::from_str("warn"), LogLevel::Warn);
        assert_eq!(LogLevel::from_str("warning"), LogLevel::Warn);
        assert_eq!(LogLevel::from_str("error"), LogLevel::Error);
        assert_eq!(LogLevel::from_str("off"), LogLevel::Off);
        assert_eq!(LogLevel::from_str("none"), LogLevel::Off);
        assert_eq!(LogLevel::from_str("unknown"), LogLevel::Info);
    }

    #[test]
    fn test_log_level_ordering() {
        assert!(LogLevel::Debug > LogLevel::Info);
        assert!(LogLevel::Info > LogLevel::Warn);
        assert!(LogLevel::Warn > LogLevel::Error);
        assert!(LogLevel::Error > LogLevel::Off);
    }
}
