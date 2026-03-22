fn main() {
    println!("Hello, world!");
}

#[cfg(test)]
mod tests {
    #[test]
    fn main_runs_without_panic() {
        super::main();
    }
}
