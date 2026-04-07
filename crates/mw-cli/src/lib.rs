//! Library half of the `mw` binary.
//!
//! The CLI dispatcher in [`commands`] is exposed here so its pure helpers
//! (everything that doesn't call `std::process::exit` or touch the real
//! filesystem) can be unit tested without spawning a subprocess.

pub mod commands;
