package main

import (
	"astra/astra/sources/identity"
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

func runSignup(args []string) {
	flags := flag.NewFlagSet("signup", flag.ContinueOnError)
	name := flags.String("name", "", "your display name")
	email := flags.String("email", "", "optional email")
	password := flags.String("password", "", "password (prefer interactive entry)")
	if err := flags.Parse(args); err != nil {
		return
	}
	if strings.TrimSpace(*name) == "" || *password == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
			fmt.Println("Signup requires --name and --password when stdin is not a terminal.")
			return
		}
		promptName, promptEmail, promptPassword, err := identity.PromptCredentials(true, true)
		if err != nil {
			fmt.Println("Signup failed: " + err.Error())
			return
		}
		if *name == "" {
			*name = promptName
		}
		if *email == "" {
			*email = promptEmail
		}
		if *password == "" {
			*password = promptPassword
		}
	}
	profile, err := identity.Default().Signup(*name, *email, *password)
	if err != nil {
		fmt.Println("Signup failed: " + err.Error())
		return
	}
	fmt.Printf("Astra profile created for %s. You are now logged in.\n", profile.Name)
	fmt.Println("Private profile: " + identity.Default().Root())
}

func runLogin(args []string) {
	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	identifier := flags.String("name", "", "name or email")
	password := flags.String("password", "", "password (prefer interactive entry)")
	if err := flags.Parse(args); err != nil {
		return
	}
	if *identifier == "" || *password == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
			fmt.Println("Login requires --name and --password when stdin is not a terminal.")
			return
		}
		fmt.Print("Name or email: ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Println("Login failed: " + err.Error())
			return
		}
		*identifier = strings.TrimSpace(line)
		fmt.Print("Password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			fmt.Println("Login failed: " + err.Error())
			return
		}
		*password = string(passwordBytes)
	}
	if err := identity.Default().Login(*identifier, *password); err != nil {
		fmt.Println("Login failed: " + err.Error())
		return
	}
	profile, _ := identity.Default().Profile()
	fmt.Printf("Logged in as %s.\n", profile.Name)
}

func runLogout() {
	if err := identity.Default().Logout(); err != nil {
		fmt.Println("Logout failed: " + err.Error())
		return
	}
	fmt.Println("Logged out of Astra. Your private profile remains on this machine.")
}

func runWhoAmI() {
	profile, err := identity.Default().LoggedIn()
	if err != nil {
		fmt.Println("Not logged in. Run `astra signup` or `astra login`.")
		return
	}
	fmt.Printf("Name: %s\n", profile.Name)
	if profile.Email != "" {
		fmt.Printf("Email: %s\n", profile.Email)
	}
	fmt.Printf("Profile: %s\n", profile.ID)
}

func requireAstraLogin() bool {
	if _, err := identity.Default().LoggedIn(); err == nil {
		return true
	} else {
		fmt.Println("Astra is locked. Sign up or log in before using the agent.")
		fmt.Println("  astra signup")
		fmt.Println("  astra login")
		return false
	}
}
