package main

import (
	"fmt"
	"strconv"
	"strings"
	"math"
)
//同一个path如果close的话 就要fill他， 中间要check有没有contain 只能contain自己的

type Point struct {
	x int
	y int
}
type Line struct {
    public_key string
	Point1 Point
	Point2 Point
}
type Path struct {
	Lines  []Line
	isClosed bool
}

var ll []Line
func parseSVG(key string, path string, i int, p Point, l []Line) (LineSet []Line) {
	if len(ll) < len(l) {
		ll = l
	}
	path_list := strings.Split(path, " ")
	total_lenth := len(path_list)
	if i >= total_lenth {
		fmt.Println(l)
		return l
	}
	temp := path_list[i]
	var tempPoint Point
	if string(temp[0]) == "M" {
		tempx, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l
		}
		tempPoint.x = tempx
		tempy, err := strconv.Atoi(path_list[i+2])
		if err != nil {
			return l
		}
		tempPoint.y = tempy
		if i+3 <= total_lenth {
			parseSVG(key, path, i+3, tempPoint, LineSet)
		} else {
			return l
		}
	}
	if string(temp[0]) == "m" {
		tempx, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l
		}
		tempPoint.x = tempx + p.x
		tempy, err := strconv.Atoi(path_list[i+2])
		if err != nil {
			return l
		}
		tempPoint.y = tempy + p.y
		if i+3 <= total_lenth {
			parseSVG(key, path, i+3, tempPoint, LineSet)
		} else {
			return l
		}
	}
	if string(temp[0]) == "L" {
		tempx, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l
		}
		tempPoint.x = tempx
		tempy, err := strconv.Atoi(path_list[i+2])
		if err != nil {
			return l
		}
		tempPoint.y = tempy
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)
		if i+3 <= total_lenth {
			parseSVG(key, path, i+3, tempPoint, LineSet)
		} else {
			return l
		}
	}
		if string(temp[0]) == "l" {
		tempx, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l
		}
		tempPoint.x = tempx + p.x
		tempy, err := strconv.Atoi(path_list[i+2])
		if err != nil {
			return l
		}
		tempPoint.y = tempy + p.y
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)
		if i+3 <= total_lenth {
			parseSVG(key, path, i+3, tempPoint, LineSet)
		} else {
			return l
		}
	}
	if string(temp[0]) == "h"{
		v, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l
		}
		tempPoint.x = v + p.x
		tempPoint.y = p.y
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)
		if i+2 <= total_lenth {
			parseSVG(key, path, i+2, tempPoint, LineSet)
		} else {
			return l
		}
	}
		if string(temp[0]) == "H" {
		v, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l
		}
		tempPoint.x = v
		tempPoint.y = p.y
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)
		if i+2 <= total_lenth {
			parseSVG(key, path, i+2, tempPoint, LineSet)
		} else {
			return l
		}
	}
	if string(temp[0]) == "v" {
		v, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l
		}
		tempPoint.x = p.x
		tempPoint.y = v + p.y
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)
		if i+2 <= total_lenth {
			parseSVG(key,path, i+2, tempPoint, LineSet)
		} else {
			return l
		}
	}
		if string(temp[0]) == "V" {
		v, err := strconv.Atoi(path_list[i+1])
		if err != nil {
			return l
		}
		tempPoint.x = p.x
		tempPoint.y = v
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)
		if i+2 <= total_lenth {
			parseSVG(key,path, i+2, tempPoint, LineSet)
		} else {
			return l
		}
	}
	if string(temp[0]) == "z" || string(temp[0]) == "Z" {
		tempx, err := strconv.Atoi(path_list[1])
		if err != nil {
			return l
		}
		tempPoint.x = tempx
		tempy, err := strconv.Atoi(path_list[2])
		if err != nil {
			return l
		}
		tempPoint.y = tempy
		tempLine := makeLine(key, p, tempPoint)
		LineSet = append(l, tempLine)
		if i+1 <= total_lenth {
			parseSVG(key,path, i+1, tempPoint, LineSet)
		} else {
			return l
		}
	}
	if total_lenth == i {
		return l
	}
	return l
}

func makeLine(tkey string, p1 Point, p2 Point) Line {
	var l Line
    l.public_key = tkey
	l.Point1 = p1
	l.Point2 = p2
	return l
}

var total_line []Line

func line_adder(ls []Line) {
 for i := 0; i <= len(ll)-1; i++ {
  total_line = append(total_line, ls[i])
  }
}
//如果在里面就return true
func contain_detector(p Point) bool{
	tlen := len(path_set)-1
	for i:=0 ;i<= tlen ;i++{
		if(path_set[i].isClosed ==true){
			if PointInPoly(p,path_set[i])==true{
				fmt.Println("jfnadkjlfnkdjahfoamf;lkasdjkasnflkasmnflmsa;lfsa")
				return true
			}
		}
	}
	return false

}
func PointInPoly(pt Point, poly Path) bool{ 
   i := len(poly.Lines)-1
   j := len(poly.Lines)-1
   oddNodes := false
   corners := getPoints(poly.Lines)
   x := pt.x
   y := pt.y
   for i=0; i<len(corners)-1; i++{
   	  polyXi := corners[i].x
   	  polyYi := corners[i].y
   	  polyXj := corners[j].x
   	  polyYj := corners[j].y

   	 if (polyYi< y && polyYj>=y || polyYj< y && polyYi>=y) && (polyXi<=x || polyXj<=x){
      oddNodes = (oddNodes|| (polyXi+(y-polyYi)/(polyYj-polyYi)*(polyXj-polyXi)<x)) && !(oddNodes && (polyXi+(y-polyYi)/(polyYj-polyYi)*(polyXj-polyXi)<x))
  }
    j=i; 
}
    fmt.Println("在里面吗", oddNodes)
    return oddNodes
}

var path_set []Path
//return true if the path do not overlap, else false
func overlap_detector(isTransparent bool, key string, path string) bool {
	ll = ll[:0]
	p := Point{0, 0}
	var l []Line
	parseSVG(key, path, 0, p, l)
	for i := 0; i <= len(ll)-1; i++ {
    if(len(total_line)== 0){
      line_adder(ll)
      return true
    }
      line_adder(ll)
      res := isValid(ll[i], total_line, isTransparent)
		  if res == false {
		  	return false
     }
	  }
	var path_temp Path 
	if isClosed(ll){
		path_temp.isClosed = true
	}else{
		path_temp.isClosed = false
	}
	for v := 0; v <= len(ll)-1; v++ {
		path_temp.Lines = append(path_temp.Lines,ll[v])
	}
	path_set = append(path_set,path_temp)
	fmt.Println("路径表",path_set)
	if contain_detector(ll[0].Point1){
		fmt.Println("在里面")
	}
	return true
}

func isValid(l Line, ls []Line, isTransparent bool) bool {
	for i := 0; i <= len(ll)-1; i++ {
		if isInter(l, ls[i], isTransparent) {
			return false
		}
	}
	return true
}

//return false if two lines do not intersact or they are same writer and no color
func isInter(l1 Line, l2 Line, isTransparent bool) bool {
	point1_x := l1.Point1.x
	point1_y := l1.Point1.y
	point2_x := l1.Point2.x
	point2_y := l1.Point2.y
    key1 := l1.public_key
	linePoint1_x := l2.Point1.x
	linePoint1_y := l2.Point1.y
	linePoint2_x := l2.Point2.x
	linePoint2_y := l2.Point2.y
    key2 := l2.public_key
  if(key1 == key2 && isTransparent == true){
    return false
  }
	var denominator = (point1_y-point2_y)*(linePoint1_x-linePoint2_x) - (point2_x-point1_x)*(linePoint2_y-linePoint1_y)
	if denominator == 0 {
		return false
	}
	var x = ((point1_x-point2_x)*(linePoint1_x-linePoint2_x)*(linePoint2_y-point2_y) +
		(point1_y-point2_y)*(linePoint1_x-linePoint2_x)*point2_x - (linePoint1_y-linePoint2_y)*
		(point1_x-point2_x)*linePoint2_x) / denominator
	var y = -((point1_y-point2_y)*(linePoint1_y-linePoint2_y)*(linePoint2_x-point2_x) +
		(point1_x-point2_x)*(linePoint1_y-linePoint2_y)*point2_y - (linePoint1_x-linePoint2_x)*
		(point1_y-point2_y)*linePoint2_y) / denominator
	if (x-point2_x)*(x-point1_x) <= 0 && (y-point2_y)*(y-point1_y) <= 0 && (x-linePoint2_x)*(x-linePoint1_x) <= 0 &&
		(y-linePoint2_y)*(y-linePoint1_y) <= 0 {
		return true
	}
  return false
}

func isClosed(lines []Line) bool{
	lastp := len(lines)-1
	if lines[0].Point1 == lines[lastp].Point2{
		return true
	}
	return false
}
func calculatelength(ls []Line) int{
	var tlength float64
	tlength = 0
	for i:=0;i<=len(ls)-1;i++ {
		cline := ls[i]
		p1 := cline.Point1
		p2 := cline.Point2
		d := math.Sqrt(float64((p1.x-p2.x)*(p1.x-p2.x)+ (p1.y-p2.y)*(p1.y-p2.y)))
		tlength = tlength + d
	}
	return int(tlength)
}

func calculateInk (path string,isTransparent bool,key string) (ink_amount int){
	notOverlap := overlap_detector(isTransparent,key,path)
	fmt.Println("1111",notOverlap)
	if !isClosed(ll){
		return calculatelength(ll)
	}
	if notOverlap == false{
		if isTransparent == true{
			ink_amount = calculatelength(ll)
		}else{
			fmt.Println("err: overlaping shapes cant be filled ")
		}
	}else{
		if isTransparent == true{
			ink_amount = calculatelength(ll)
		}else{
			area := calculateArea(ll)
			ink_amount = area
		}
	}
	return ink_amount
}

func det(p0 Point,p1 Point) (res int) {
	res += p0.x * p1.y;  
    res -= p0.y * p1.x;  
    return res
}  

func getPoints(linelist []Line) (points []Point){
	points = append(points,linelist[0].Point1)
	for i:=0; i<len(linelist);i++{
		points= append(points,linelist[i].Point2)
	}
	return points
}

func calculateArea(linelist []Line) (area int){
	pointlist := getPoints(linelist)
	totalPoints := len(pointlist)
    area = 0
	for i:=1; i<totalPoints-1; i++ {
		p1 := pointlist[i-1]
		p2 := pointlist[i]
        area += det(p1,p2);  
		}
    temp := float64(area)
    area = int(math.Abs(temp/2))
	return area
}

func main() {
   var temp_key1 = "123456"
   var temp_key2 = "234567"
  var temp_key4 = "345678"

   fmt.Println("对",overlap_detector(true, temp_key1, "m 0 100 v 100 h 200 l 100 200 z"))
   fmt.Println("对",overlap_detector(true, temp_key2, "m 100 10 v 1000"))
   fmt.Println("错",overlap_detector(true, "1", "m 500 400 l -500 -300"))
   fmt.Println("对",overlap_detector(true, "11", "m 300 150 h 100"))
   fmt.Println("对",overlap_detector(true, "111", "M 400 400 v 100 h 100 z"))
   fmt.Println("里面测试",overlap_detector(true, "111", "m 410 420 l 10 10"))

  fmt.Println("面积墨水",calculateInk("M 400 400 v 100 h 100 z", false ,temp_key4))
}
