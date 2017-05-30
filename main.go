// healthServer project main.go
package main

import (
    "fmt"
	"reflect"
    "net"
    "log"
    "net/http"
    "time"
    "encoding/json"
    "github.com/gorilla/mux"
	proc "github.com/c9s/goprocinfo/linux"
	"os"
    "github.com/spf13/viper"
)


//Global scope declaration made for var newhost
var newhost = make(map[string]interface{})


/* A Simple function to check errors */
func CheckError(err error) {
    if err  != nil {
        log.Println("Error: " , err)
		log.Fatal(err)
        os.Exit(0)
    }
}
//Could be used to count members
/*func stringInSlice(a string, list []string) bool {
    for _, b := range list {
        if b == a {
            return true
        }
    }
    return false
}*/

func sendSysinfoMulticast(a string, messages chan interface{}) {
	addr, err := net.ResolveUDPAddr("udp", a)
	CheckError(err)
	c, err := net.DialUDP("udp", nil, addr)
	CheckError(err)
	for {
		msg := <-messages
		str, err := json.Marshal(msg)
		if err != nil {
            fmt.Println("Error encoding JSON")
        return
        }
        //Write to bytes to multicast UDP
	    c.Write(str)
		time.Sleep(5 * time.Second)
	}
}

//sysinfo is looping forever in a Go routine
func sysinfo (messages chan interface{}) {
	for{
		hostname,err := os.Hostname()
	    CheckError(err)

	    //getting infos and json it !
        sstat, _ := proc.ReadStat("/proc/stat")
        sdiskstats, _ := proc.ReadDiskStats("/proc/diskstats")
        sloadavg, _ := proc.ReadLoadAvg("/proc/loadavg")
        smeminfo,_ := proc.ReadMemInfo("/proc/meminfo")
        smounts, _ := proc.ReadMounts("/proc/mounts")
        snetstat, _ := proc.ReadNetStat("/proc/net/netstat")
        sdevstat, _ := proc.ReadNetworkStat("/proc/net/dev")
        ssockstat, _ := proc.ReadSockStat("/proc/net/sockstat")
        svmstat, _ := proc.ReadVMStat("/proc/vmstat")

        type Info struct {
			Hostname   string
            Stat       interface{}
            Diskstats  interface{}
            Loadavg    interface{}
            Meminfo    interface{}
            Mounts     interface{}
            Netstat    interface{}
            Devstat    interface{}
            Sockstat   interface{}
	        Vmstat     interface{}
        }

	    infos := map[string]interface{}{
        "stat" : sstat,
        "diskstats" : sdiskstats,
        "loadavg" : sloadavg,
        "meminfo" : smeminfo,
        "mounts" : smounts,
        "netstat" : snetstat,
        "devstat" : sdevstat,
        "sockstat" : ssockstat,
        "vmstat" : svmstat,
        }
		m := make(map[string]interface{})
		m["hostname"] = hostname
		m["metrics"] = infos
	    messages <- m //send in chan
	    }
}

func reply(w http.ResponseWriter, infos chan interface{}, members []string){
	log.Println(reflect.TypeOf(infos))
	info := <-infos
	m := info.(map[string]interface{})
	hostname := m["hostname"]
	delete(m,"hostname")
	//Here we could count for members
	//if stringInSlice(hostname.(string),members){
	//members = append(members, hostname.(string))
	//}
	//numberMembers := len(members)
	newhost[hostname.(string)] = m["metrics"]
	json.NewEncoder(w).Encode(newhost)
}

func main() {
    viper.SetConfigName("config") // name of config file (without extension)
    viper.AddConfigPath("/etc/healthserver/")   // path to look for the config file in
	err := viper.ReadInConfig() // Find and read the config file
    if err != nil { // Handle errors reading the config file
        panic(err)
    }
	viper.WatchConfig()
    srvAddr := viper.Get("MulticastIPandPort")	
	maxDatagramSize := viper.Get("maxDatagramSize")
	ListenPort := viper.Get("ListenPort")
	TimeRefresh := viper.Get("TimeRefresh")
	f, err := os.OpenFile("/var/log/healthserver.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
    if err != nil {
        panic(err)
    }
    defer f.Close()
    log.SetOutput(f)
	log.Printf("Starting with configuration set MulticastIPandPort: %+v, maxDatagramSize: %+v, ListenPort: %+v, TimeRefresh: %+v.",srvAddr,maxDatagramSize,ListenPort,TimeRefresh)
	var members []string
    messages := make(chan interface{})
	go sysinfo(messages)
	go sendSysinfoMulticast(srvAddr.(string), messages)
	go serveMulticastUDP(srvAddr.(string),messages,maxDatagramSize.(int))//, msgHandler)
    router := mux.NewRouter().StrictSlash(true)
    router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){
	reply(w,messages,members)
    })
    log.Fatal(http.ListenAndServe(ListenPort.(string), router))
}


/*func msgHandler(src *net.UDPAddr, n int, b []byte) {
	log.Println(n, "bytes read from", src)
	s := string(b[:n])//convert my buffer of bytes to string for printing out
	log.Println(s)
}*/

//func serveMulticastUDP(a string, h func(*net.UDPAddr, int, []byte)) {
func serveMulticastUDP(a string, messages chan interface{}, maxDatagramSize int) {

	addr, err := net.ResolveUDPAddr("udp", a)
	CheckError(err)
	l, err := net.ListenMulticastUDP("udp", nil, addr)
	l.SetReadBuffer(maxDatagramSize)
	for {
		b := make([]byte, maxDatagramSize)
		n, src, err := l.ReadFromUDP(b)
		if err != nil {
			log.Fatal("ReadFromUDP failed:", err)
		}
		var fromMulticast map[string]interface{}
        if err := json.Unmarshal(b[:n], &fromMulticast); err != nil {
			log.Println(n, string(b[:n]), b[:n])
			//log.Printf("%+v", fromMulticast)
            panic(err)
        }
		messages<-fromMulticast
		//log.Println(fromMulticast)
		log.Println(n, "bytes read from", src)
		//log.Println(src)
		//log.Println(n)
		//h(src, n, b)
	}
}
