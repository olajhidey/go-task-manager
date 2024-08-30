new Vue({
    el: '#app',
    data: {
        tasks: [],
        taskForm: {
            name: '',
            description: '',
            runAt: '',
            status: 'Pending'
        },
        editIndex: null
    },
    mounted(){
        this.getTasks()
    },
    methods: {
       async saveTask() {
            if (this.editIndex === null) {
                const request = await fetch("/api/task/create", {
                    method: 'POST',
                    body: JSON.stringify({
                        name: this.taskForm.name, 
                        description: this.taskForm.description,
                        runAt: this.taskForm.duedate,
                        status: "Scheduled"
                    })
                })

                if(!request.ok){
                    throw new Error("Something went wrong", request)
                }

                const response = await request.json()
                this.tasks.push(response)
                // this.tasks.push({ ...this.taskForm });
            } else {
                Vue.set(this.tasks, this.editIndex, { ...this.taskForm });
                this.editIndex = null;
            }
            this.resetForm();
        },
        editTask(index) {
            this.taskForm = { ...this.tasks[index] };
            this.editIndex = index;
        },
        async getTasks(){

            // Get all of the tasks 
            const response = await fetch("/api/tasks", {
                method: 'GET'
            })

            const data = await response.json()
            if (data != null){
                this.tasks = []
            }
            console.log(data)
            this.tasks = data
        }, 
        async deleteTask(task, index) {

            this.tasks.splice(index, 1);
            const url = "/api/task/delete/"+task
            const request = await fetch(url, {
                method: 'DELETE'
            })

            if(!request.ok){
                throw new Error("Something went wrong", request)
            }

            const response = await request.json()
            console.log(response)
        },
        resetForm() {
            this.taskForm = {
                name: '',
                description: '',
                duedate: '',
                status: 'Pending'
            };
        },
        statusClass(status) {
            return {
                'badge-pending': status === 'Scheduled',
                'badge-completed': status === 'Completed'
            };
        }
    }
});
