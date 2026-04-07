use clap::Parser;
use mw_cli::commands::{self, Cli};

fn main() {
    let cli = Cli::parse();
    commands::dispatch(cli);
}
