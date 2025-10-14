package main

// fmt.Println("heck database support ipv4:", db.IsIPv4())     // check database support ip type
// fmt.Println("check database support ip type:", db.IsIPv6()) // check database support ip type
// fmt.Println("database build time:", db.BuildTime())         // database build time
// fmt.Println("database support language:", db.Languages())   // database support language
// fmt.Println("database support fields:", db.Fields())        // database support fields
func main() {

	conf.init()

	go StartRecoder()

	go newplay()

	go startcron()

	go play()

	udpClient()
}
