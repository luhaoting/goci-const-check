package main

import (
	pb "goci-const-check/pb"
)

func main() {
	t := &pb.Person{}
	t.Id = 12345
	t.Name = "Alice"
	t.Age = 30

	println(t.Id, t.Name, t.Age)

	School := &pb.School{}
	School.Name = "Sunshine High"
	School.Address = "123 Main St"
	team := &pb.TeacherTeam{
		Teachers: map[uint32]*pb.Person{
			1: {Id: 1, Name: "Mr. Smith", Age: 40},
			2: {Id: 2, Name: "Ms. Johnson", Age: 35},
		},
	}
	School.Teachers = team

	team2 := &pb.TeacherTeam{
		Teachers: map[uint32]*pb.Person{
			3: {Id: 3, Name: "Mr. Lee", Age: 40},
			4: {Id: 4, Name: "Ms. Cool ", Age: 35},
		},
	}

	School.Teachers = team2

	School.Teachers.Teachers[5] = &pb.Person{Id: 5, Name: "Ms. New", Age: 29}
}
