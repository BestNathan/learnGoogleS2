package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/golang/geo/s2"
)

type point struct {
	coors []float64
	index int
	name  string
}

type cellPoint struct {
	cellType string
	names    []string
	coor     []float64
	count    int
}

var points = []point{}

func generatePoints() {
	for index := 0; index < 500000; index++ {
		p := point{
			coors: []float64{(1.5 * rand.Float64()) + 116, (1.5 * rand.Float64()) + 39},
			index: index,
			name:  fmt.Sprintf("Point%d", index),
		}
		points = append(points, p)
	}
}

type query struct {
	Bounds string `form:"bounds"`
	Zoom   int    `form:"zoom"`
	Rect   s2.Rect
	Cover  s2.CellUnion
}

func (q *query) getPoints() []gin.H {
	q.init()
	q.cover()
	r := []gin.H{}
	// for _, point := range points {
	// 	coors := point.coors
	// 	if q.Rect.ContainsLatLng(s2.LatLngFromDegrees(coors[1], coors[0])) {
	// 		l++
	// 		if len(r) > 100 {
	// 			continue;
	// 		}
	// 		r = append(r, gin.H{
	// 			"position": coors,
	// 			"tip":      point.name,
	// 		})
	// 	}
	// }
	cmap := make(map[string]*cellPoint)
	var wg sync.WaitGroup
	var m sync.Mutex
	wg.Add(len(points))
	for _, p := range points {
		go func (pp point) {
			defer wg.Done()
			coors := pp.coors
			// 转化点
			ll := s2.LatLngFromDegrees(coors[1], coors[0])
			p := s2.PointFromLatLng(ll)

			if !q.Rect.ContainsPoint(p) {
				return
			}

			// 判断点是否在内
			if cell, ok := pointIsInCellUnion(p, q.Cover); ok {
				cstr := cell.String()
				m.Lock()
				cp, cpok := cmap[cstr]
				if cpok {
					cp.names = append(cp.names, pp.name)
					cp.count++
					if cp.count > 1 && cp.cellType == "device" {
						cp.cellType = "cluster"

						centerP := s2.LatLngFromPoint(s2.CellFromCellID(cell).Center())
						cp.coor = []float64{centerP.Lng.Degrees(), centerP.Lat.Degrees()}
					}
				} else {
					

					cp = &cellPoint{
						coor: coors,
						cellType: "device",
						count: 1,
					}
					cp.names = append(cp.names, pp.name)

					cmap[cstr] = cp
				}
				m.Unlock()
			}
		}(p)
	}

	wg.Wait()
	for _, cp := range cmap {
		r = append(r, gin.H{
			"type": cp.cellType,
			"count": cp.count,
			"devices": cp.names,
			"position": cp.coor,
		})
	}
	return r
}

func pointIsInCellUnion(p s2.Point, cu s2.CellUnion) (s2.CellID, bool) {
	for _, cell := range cu {
		if s2.CellFromCellID(cell).ContainsPoint(p) {
			return cell, true
		}
	}
	return s2.CellID(0), false
}

func (q *query) init() {
	q.Rect = buildRect(q.Bounds)
}

func (q *query) cover() {
	mc := 10 * (q.Zoom - 1)
	defaultCoverer := &s2.RegionCoverer{MaxLevel: q.Zoom + 1, MaxCells: mc, MinLevel: q.Zoom - 1}
	q.Cover = defaultCoverer.Covering(q.Rect)
	// fmt.Printf("--cvr----%d------\n", len(cvr))
}

func buildRect(bounds string) s2.Rect {
	ps := [][]float64{}
	strs := strings.Split(bounds, ";")
	for _, str := range strs {
		coors := strings.Split(str, ",")
		p := []float64{}
		for _, coor := range coors {
			n, _ := strconv.ParseFloat(coor, 10)
			p = append(p, n)
		}
		ps = append(ps, p)
	}

	sw := s2.LatLngFromDegrees(ps[0][1], ps[0][0])
	ne := s2.LatLngFromDegrees(ps[1][1], ps[1][0])
	return s2.EmptyRect().AddPoint(sw).AddPoint(ne)
}

func main() {
	generatePoints()
	g := gin.Default()
	c := cors.DefaultConfig()
	c.AllowCredentials = true
	c.AllowAllOrigins = true
	g.Use(cors.New(c))
	g.GET("/map", func(c *gin.Context) {
		var q query
		c.Bind(&q)

		t := time.Now()
		ps:= q.getPoints()
		fmt.Printf("aggregate points time: %f S\n", time.Since(t).Seconds())

		c.JSON(200, gin.H{
			"points": ps,
			"counts": len(ps),
		})
	})

	g.Run(":8080")
}
