mod commands;

use clap::Parser;
use commands::Cli;

fn main() {
    let cli = Cli::parse();
    commands::dispatch(cli);
}
